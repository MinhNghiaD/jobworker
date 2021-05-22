package log

import (
	"fmt"
)

// LogReader is the interface that allow a stream to retrieve enties of a process log
type LogReader interface {
	// ReadLine reads an entry in the log file
	// If there is no more data to read, it will wait until there is new entry in the file
	// If the file is closed, ReadLine returns an empty string with an EOF error
	ReadLine() (string, error)
	// Close closes reader and release its resources
	Close() error
}

// ReaderImpl implements the functionalities of LogReader interface
type ReaderImpl struct {
}

// ReadLine reads the next line in the log file. If the logging is finished or the reading is cancel by its context, ReadLine will return with an error
func (reader *ReaderImpl) ReadLine() (string, error) {
	return "", fmt.Errorf("Unimplemented")
}

// Close closes the reader and freeup its resources
func (reader *ReaderImpl) Close() error {
	return fmt.Errorf("Unimplemented")
}
