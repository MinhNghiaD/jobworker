package log

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/google/uuid"
)

// Reader is the interface that allow a stream to retrieve enties of a process log
type Reader interface {
	// ReadAt reads an entry in the log file at a given index
	// If there is no more data to read, it will wait until there is new entry in the file
	// If the file is closed, ReadLine returns an empty string with an EOF error
	ReadAt(index int) (string, error)
	// Close closes reader and release its resources
	Close() error
}

// ReaderImpl implements the functionalities of LogReader interface
type ReaderImpl struct {
	fileName  string
	hook      *EventHook
	id        string
	eventChan <-chan bool
	ctx       context.Context
}

// newLogReader creates a new LogReader, note that this interface is not thread-safe, it can only be used by only one stream
func newLogReader(ctx context.Context, fileName string, hook *EventHook) (Reader, error) {
	id := uuid.New().String()
	eventChan, err := hook.subscribe(id)
	if err != nil {
		return nil, fmt.Errorf("Fail to init subscribe to event hook")
	}

	return &ReaderImpl{
		fileName:  fileName,
		hook:      hook,
		id:        id,
		eventChan: eventChan,
		ctx:       ctx,
	}, nil
}

// ReadAt reads line at a given index in the log file.
// If the index is out of range, ReadAt will wait until the entry of this index is written.
// If there is no more log be written and the index is still out of range, ReadAt return an EOF error.
func (reader *ReaderImpl) ReadAt(index int) (string, error) {
	file, err := os.Open(reader.fileName)
	if err != nil {
		return "", err
	}

	defer file.Close()

	var data []byte
	var entryOffset *EntryOffset = nil

	entryOffset, err = reader.hook.lookupEntry(index)
	if entryOffset != nil {
		data = make([]byte, entryOffset.size)
		_, err = file.ReadAt(data, entryOffset.offset)
	}

	if err != nil && err != io.EOF {
		return "", err
	}

	for err == io.EOF {
		if err = reader.wait(); err != nil {
			return "", err
		}

		// TODO handle log rotation
		if entryOffset == nil {
			entryOffset, err = reader.hook.lookupEntry(index)
		}

		if entryOffset != nil {
			data = make([]byte, entryOffset.size)
			_, err = file.ReadAt(data, entryOffset.offset)
		}
	}

	return string(data), err
}

// Close closes the reader and unsubscribe its from the EventHook, closing its event channel.
func (reader *ReaderImpl) Close() error {
	return reader.hook.unsubscribe(reader.id)
}

// wait waits for the log file is available to continue reading or the read is cancelled.
func (reader *ReaderImpl) wait() error {
	for {
		select {
		case <-reader.ctx.Done():
			return fmt.Errorf("Read cancelled")
		case e, ok := <-reader.eventChan:
			if !ok || !e {
				return io.EOF
			}
			return nil
		}
	}
}
