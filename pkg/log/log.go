package log

import (
	"os"

	"github.com/sirupsen/logrus"
)

type Logger struct {
	file  *os.File
	Entry *logrus.Entry
}

func (logger *Logger) Close() error {
	return nil
}

type LogsManager struct {
	logsDir string
}

func NewManager() (*LogsManager, error) {
	return &LogsManager{}, nil
}

func (manager *LogsManager) NewLogger(ID string) (*Logger, error) {
	return &Logger{}, nil
}

func (manager *LogsManager) Cleanup() error {
	return nil
}
