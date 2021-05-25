package log

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Logger wraps of logrus logging into a file.
// Logger also includes the EventHook that attached to the Logger in order to receive file event.
type Logger struct {
	file  *os.File
	Entry *logrus.Entry
	hook  *EventHook
}

// Close closes loggers and its log file.
// It also close the EventHook which results in the EOF notification to all its subscribers
func (logger *Logger) Close() error {
	if err := logger.file.Close(); err != nil {
		logrus.Errorf("Fail to close file %s, error: %s", logger.file.Name(), err)
		return err
	}

	logger.hook.close()
	logrus.Debugf("Close logger %s", logger.file.Name())

	return nil
}

// NewReader returns a new log reader that can read this log file. This reader is a subscriber of the EventHook.
// It gains access to line offset lookup and file event notification.
func (logger *Logger) NewReader(ctx context.Context) (Reader, error) {
	return newLogReader(ctx, logger.file.Name(), logger.hook)
}

// LogsManager manages logs directory and the creation of loggers
type LogsManager struct {
	logsDir string
}

// NewManager creates a new logs manager with a temporary log directory.
// It is the access point to internal logging of job worker service.
func NewManager() (*LogsManager, error) {
	// TODO use config file to configure logs directory
	logsDir, err := os.MkdirTemp("", "worker-logs-*")
	if err != nil {
		return nil, err
	}

	logrus.Infof("Log files are stored in %s", logsDir)

	return &LogsManager{
		logsDir: logsDir,
	}, nil
}

// NewLogger creates a new logger with a log file named after the ID, in the logs directory.
func (manager *LogsManager) NewLogger(ID string) (*Logger, error) {
	if len(ID) == 0 {
		return nil, fmt.Errorf("ID cannot be empty")
	}

	formatter := &logrus.JSONFormatter{
		TimestampFormat: "02-01-2006 15:04:05",
	}

	fileName := filepath.Join(manager.logsDir, fmt.Sprintf("%s.log", ID))

	// Create write only log file.
	file, err := os.OpenFile(fileName, (os.O_CREATE | os.O_APPEND | os.O_WRONLY), 0644)
	if err != nil {
		return nil, err
	}

	logger := logrus.Logger{
		Out:       file,
		Formatter: formatter,
		Hooks:     make(logrus.LevelHooks),
		Level:     logrus.InfoLevel,
	}

	// Create event hook to receive file event from logger.
	eventHook := &EventHook{
		subscribers: make(map[string]chan bool),
		lineOffset:  make([]*EntryOffset, 0, 1024),
		nextLinePos: 0,
		isClosed:    false,
	}
	logger.AddHook(eventHook)

	return &Logger{
		file:  file,
		Entry: logger.WithField("source", "log"),
		hook:  eventHook,
	}, nil
}

// Cleanup removes the temporary logs directory
func (manager *LogsManager) Cleanup() error {
	if err := os.RemoveAll(manager.logsDir); err != nil {
		logrus.Infof("Failed to remove %q: %v", manager.logsDir, err)
	}

	return nil
}

// EntryOffset encapsulates the offset of the log entry in the file, as well as it length.
type EntryOffset struct {
	offset int64
	size   int
}

// EventHook is a hook designed for listening log event.
// EventHook is added to the logger in order to receive the notification when a new log entry is written.
// The connection between logger and hook is protected by the built-in mutex of logrus.
// In the event of new log entry is written, EventHook records the offset and the length of this entry in the log file.
// This allow us to perfrom fast seek while reading file. With this lookup, Reader can treat the log file as a buffer.
// EventHook is also treated as a event hub, where Reader can subscribe and receive a broadcast of file writing event.
type EventHook struct {
	// Map subscriber-channel, we will send true to the channel in the event of a write, and false in the event of log closing
	subscribers map[string](chan bool)
	// lineOffset indexes the offset and the length of lines in the log file
	lineOffset []*EntryOffset
	// nextLinePos points to the end of file which will be the offset of the next line
	nextLinePos int64
	isClosed    bool
	eventMutex  sync.RWMutex
	offsetMutex sync.RWMutex
}

// Fire receives write signal from logger, it will send write event to all its subscribers
func (hook *EventHook) Fire(e *logrus.Entry) error {
	hook.eventMutex.RLock()
	defer hook.eventMutex.RUnlock()

	if hook.isClosed {
		return fmt.Errorf("Event Hook is closed")
	}

	s, err := e.String()
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		// Index entry offset in the file
		defer wg.Done()
		hook.offsetMutex.Lock()
		hook.lineOffset = append(hook.lineOffset, &EntryOffset{hook.nextLinePos, len(s)})
		hook.nextLinePos += int64(len(s))
		hook.offsetMutex.Unlock()
	}()

	// Broadcast event to the subscribers
	for _, sub := range hook.subscribers {
		wg.Add(1)
		go func(sub chan bool) {
			defer wg.Done()

			select {
			case sub <- true:
			case <-time.After(100 * time.Millisecond):
			}
		}(sub)
	}

	wg.Wait()

	return nil
}

// Levels implement Levels function of logrus.Hook interface
func (hook *EventHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// lookupEntry allows subscribers to perform log entry lookup.
// If the entry at the index is already written to the file, lookupEntry returns the offset and the size of this entry in the log file.
// If the entry hasn't been written, lookupEntry returns an EOF error.
func (hook *EventHook) lookupEntry(index int) (*EntryOffset, error) {
	hook.offsetMutex.RLock()
	defer hook.offsetMutex.RUnlock()

	if index < len(hook.lineOffset) {
		return hook.lineOffset[index], nil
	}

	return nil, io.EOF
}

// subscribe add a subscriber and gives it an event channel to receive file event
// The subscriber can then receive the file notification from this channel.
// A true value indicate a log entry is written, a false value mean that en file is close and there is no more data to read.
func (hook *EventHook) subscribe(id string) (<-chan bool, error) {
	hook.eventMutex.Lock()
	defer hook.eventMutex.Unlock()

	if _, ok := hook.subscribers[id]; ok {
		return nil, fmt.Errorf("Subscriber is already added")
	}

	hook.subscribers[id] = make(chan bool, 100)
	logrus.Infof("Add subscribe %s", id)

	if hook.isClosed {
		// Send true to channel to flush out the read before closing
		hook.subscribers[id] <- true
		hook.subscribers[id] <- false
	}

	return hook.subscribers[id], nil
}

// unsubscribe removes a subscriber and close its channel.
func (hook *EventHook) unsubscribe(id string) error {
	hook.eventMutex.Lock()
	defer hook.eventMutex.Unlock()

	if _, ok := hook.subscribers[id]; !ok {
		return fmt.Errorf("Subscriber is already added")
	}

	logrus.Infof("Unsubscribe %s", id)
	close(hook.subscribers[id])
	delete(hook.subscribers, id)

	return nil
}

// close closes the event hook and informs all its subscribers that the logger has stopped writing.
// Before notifying the closing of the file, we send a last write event to the channel to trigger the last read from the subscriber.
func (hook *EventHook) close() {
	hook.eventMutex.Lock()
	defer hook.eventMutex.Unlock()

	hook.isClosed = true

	var wg sync.WaitGroup
	for _, sub := range hook.subscribers {
		wg.Add(1)
		go func(sub chan bool) {
			defer wg.Done()
			// Send true to channel to flush out the read before closing
			sub <- true
			sub <- false
		}(sub)
	}
	wg.Wait()
}
