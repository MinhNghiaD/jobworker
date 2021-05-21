package log

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

// Logger wraps of logrus logging into a file
type Logger struct {
	file  *os.File
	Entry *logrus.Entry
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

	logrus.Debugf("Close logger %s", logger.file.Name())
	return nil
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

	return &Logger{
		file:  file,
		Entry: logger.WithField("source", "log"),
	}, nil
}

// Cleanup removes the temporary logs directory
func (manager *LogsManager) Cleanup() error {
	if err := os.RemoveAll(manager.logsDir); err != nil {
		logrus.Infof("Failed to remove %q: %v", manager.logsDir, err)
	}

	return nil
}
