package log

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// Reader is the interface that allow a stream to retrieve enties of a process log
type Reader interface {
	// ReadLine reads an entry in the log file
	// If there is no more data to read, it will wait until there is new entry in the file
	// If the file is closed, ReadLine returns an empty string with an EOF error
	ReadLine() (string, error)
	// Close closes reader and release its resources
	Close() error
}

// ReaderImpl implements the functionalities of LogReader interface
type ReaderImpl struct {
	file      *os.File
	buffer    *bufio.Reader
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

	// Open file for read-only
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}

	return &ReaderImpl{
		file:      file,
		buffer:    bufio.NewReader(file),
		hook:      hook,
		id:        id,
		eventChan: eventChan,
		ctx:       ctx,
	}, nil
}

// TODO: Using an indexing system that allow quick access to file using line index, instead of scan the entire a file for every read.
// This will speed up the continuation of an interrupted stream.

// ReadLine reads the next line in the log file. If the logging is finished or the reading is cancel by its context, ReadLine will return with an error
func (reader *ReaderImpl) ReadLine() (string, error) {
	bytes, err := reader.buffer.ReadBytes('\n')
	if err != nil && err != io.EOF {
		return "", err
	}

	for err == io.EOF {
		if err = reader.wait(); err != nil {
			return "", err
		}

		// TODO handle log rotation
		bytes, err = reader.buffer.ReadBytes('\n')
	}

	return string(bytes), err
}

// Close closes the reader and freeup its resources
func (reader *ReaderImpl) Close() error {
	if err := reader.hook.unsubscribe(reader.id); err != nil {
		logrus.Warnf("Fail to unsubscribe, error %s", err)
	}

	return reader.file.Close()
}

// wait waits for the log file is available to continue reading
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
