package log

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Logger wraps of logrus logging into a file
type Logger struct {
	file  *os.File
	Entry *logrus.Entry
	hook  *EventHook
}

// Close closes loggers and its log file
func (logger *Logger) Close() error {
	if err := logger.file.Sync(); err != nil {
		logrus.Errorf("Fail to sync file %s, error: %s", logger.file.Name(), err)
		return err
	}

	if err := logger.file.Close(); err != nil {
		logrus.Errorf("Fail to close file %s, error: %s", logger.file.Name(), err)
		return err
	}

	logger.hook.close()

	logrus.Debugf("Close logger %s", logger.file.Name())
	return nil
}

// NewReader returns a new log reader that can read this log file
func (logger *Logger) NewReader(ctx context.Context) (Reader, error) {
	return newLogReader(ctx, logger.file.Name(), logger.hook)
}

// LogsManager manages logs directory and the creation of loggers
type LogsManager struct {
	logsDir string
}

// NewManager creates a new logs manager with a temporary log directory
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

// NewLogger creates a new logger with a log file named after the ID, in the logs directory
func (manager *LogsManager) NewLogger(ID string) (*Logger, error) {
	if len(ID) == 0 {
		return nil, fmt.Errorf("ID cannot be empty")
	}

	formatter := new(logrus.JSONFormatter)
	formatter.TimestampFormat = "02-25-2001 15:04:05"

	fileName := filepath.Join(manager.logsDir, fmt.Sprintf("%s.log", ID))

	// Create write only log file.
	file, err := os.OpenFile(fileName, (os.O_CREATE | os.O_APPEND | os.O_WRONLY), 0644)
	if err != nil {
		// Cannot open log file.
		return nil, err
	}

	logger := logrus.Logger{
		Out:       file,
		Formatter: formatter,
		Hooks:     make(logrus.LevelHooks),
		Level:     logrus.InfoLevel,
	}

	// Create event hook to receive file event from logger
	eventHook := &EventHook{
		subscribers: make(map[string]chan bool),
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

// EventHook is a hook designed for listening log event
type EventHook struct {
	// Map subscriber-channel, we will send true to the channel in the event of a write, and false in the event of log closing
	subscribers map[string](chan bool)
	isClosed    bool
	mu          sync.RWMutex
}

// Fire receives write signal from logger, it will send write event to all its subscribers
func (hook *EventHook) Fire(e *logrus.Entry) error {
	hook.mu.RLock()
	defer hook.mu.RUnlock()

	if hook.isClosed {
		return fmt.Errorf("Event Hook is closed")
	}

	var wg sync.WaitGroup
	for _, sub := range hook.subscribers {
		wg.Add(1)
		go func(sub chan bool) {
			select {
			case sub <- true:
			case <-time.After(100 * time.Millisecond):
			}
			wg.Done()
		}(sub)

		wg.Wait()
	}

	return nil
}

// Levels implement Levels function of logrus.Hook interface
func (hook *EventHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// subscribe add a subscriber and gives it an event channel to receive file event
func (hook *EventHook) subscribe(id string) (<-chan bool, error) {
	hook.mu.Lock()
	defer hook.mu.Unlock()

	if _, ok := hook.subscribers[id]; ok {
		return nil, fmt.Errorf("Subscriber is already added")
	}

	hook.subscribers[id] = make(chan bool, 100)

	if hook.isClosed {
		// Send true to channel to flush out the read before closing
		hook.subscribers[id] <- true
		hook.subscribers[id] <- false
	}

	logrus.Infof("Add subscribe %s", id)
	return hook.subscribers[id], nil
}

// unsubscribe removes a subscriber and close its channel
func (hook *EventHook) unsubscribe(id string) error {
	hook.mu.Lock()
	defer hook.mu.Unlock()

	if _, ok := hook.subscribers[id]; !ok {
		return fmt.Errorf("Subscriber is already added")
	}

	logrus.Infof("Unsubscribe %s", id)
	close(hook.subscribers[id])
	delete(hook.subscribers, id)

	return nil
}

// close closes the event hook and informs all its subscribers that the logger has stopped writing
func (hook *EventHook) close() {
	hook.mu.Lock()
	defer hook.mu.Unlock()

	hook.isClosed = true

	var wg sync.WaitGroup
	for _, sub := range hook.subscribers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Send true to channel to flush out the read before closing
			sub <- true
			sub <- false
		}()
		wg.Wait()
	}
}
