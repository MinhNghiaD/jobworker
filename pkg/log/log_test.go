package log

import (
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

func TestCreateLogFiles(t *testing.T) {
	logsManager, err := NewManager()

	if err != nil {
		t.Fatal(err)
	}

	if _, err = os.Stat(logsManager.logsDir); err != nil {
		t.Error(err)
	}

	// checker
	checkFileCreated := func(logger *Logger) error {
		if _, err = os.Stat(logger.file.Name()); err != nil {
			return err
		}

		return nil
	}

	testcases := []struct {
		name      string
		input     string
		expectErr bool
	}{
		{
			"Empty ID",
			"",
			true,
		},
		{
			"Invalid name",
			"invalid/filename",
			true,
		},
		{
			"Name with space",
			"file name",
			false,
		},
		{
			"Valid name",
			"filename",
			false,
		},
		{
			"With UUID",
			uuid.New().String(),
			false,
		},
	}

	for _, testCase := range testcases {
		t.Run(testCase.name, func(t *testing.T) {
			logger, err := logsManager.NewLogger(testCase.input)

			if (err != nil) != (testCase.expectErr) {
				t.Errorf("Test case %s with input %s returns error %v, while %v error is expected", testCase.name, testCase.input, err, testCase.expectErr)
				return
			}

			if logger == nil {
				return
			}

			if (checkFileCreated(logger) != nil) != (testCase.expectErr) {
				t.Errorf("Test case %s with input %s returns error %v, while %v error is expected", testCase.name, testCase.input, err, testCase.expectErr)
				return
			}

			if err := logger.Close(); err != nil {
				t.Errorf("Fail to close logger %s", err)
				return
			}
		})
	}

	if err := logsManager.Cleanup(); err != nil {
		t.Errorf("fail to cleanup logs dir, %s", err)
	}
}

func TestWriteLogs(t *testing.T) {
	logsManager, err := NewManager()

	if err != nil {
		t.Fatal(err)
	}

	// checker
	checkLogging := func(t *testing.T, logger *Logger, level logrus.Level, messages []string) {
		logger.Entry.Logger.SetLevel(level)
		hook := test.NewLocal(logger.Entry.Logger)

		for _, msg := range messages {
			logger.Entry.Print(msg)

			if level >= logrus.InfoLevel {
				assert.Equal(t, logrus.InfoLevel, hook.LastEntry().Level)
				assert.Equal(t, msg, hook.LastEntry().Message)
			}
		}

		if level >= logrus.InfoLevel {
			assert.Equal(t, len(messages), len(hook.Entries))
		} else {
			assert.Equal(t, 0, len(hook.Entries))
		}

		hook.Reset()
		assert.Nil(t, hook.LastEntry())
	}

	testcases := []struct {
		name  string
		level logrus.Level
	}{
		{
			"Debug",
			logrus.DebugLevel,
		},
		{
			"Info",
			logrus.InfoLevel,
		},
		{
			"Warn",
			logrus.WarnLevel,
		},
		{
			"Error",
			logrus.ErrorLevel,
		},
	}

	for _, testCase := range testcases {
		t.Run(testCase.name, func(t *testing.T) {
			logger, err := logsManager.NewLogger(uuid.New().String())

			if err != nil {
				t.Errorf("Fail to create logger")
				return
			}

			messages := make([]string, 0, 10)

			for i := 0; i < 10; i++ {
				messages = append(messages, randomString(100))
			}

			checkLogging(t, logger, testCase.level, messages)

			if err := logger.Close(); err != nil {
				t.Errorf("Fail to close logger %s", err)
				return
			}
		})
	}

	if err := logsManager.Cleanup(); err != nil {
		t.Errorf("fail to cleanup logs dir, %s", err)
	}
}

func randomString(length int) string {
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}

	return string(b)
}
