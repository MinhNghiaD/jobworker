package log

import (
	"context"
	"reflect"
	"strconv"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus/hooks/test"
)

// TestLogRead test a scenario where logs are completely written by writer first, the multiple readers read the same file
func TestLogRead(t *testing.T) {
	checkReader := func(t *testing.T, logger *Logger, hook *test.Hook) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		readText := testRead(ctx, t, logger)

		if len(hook.Entries) != len(readText) {
			t.Errorf("Read content missing %d != %d", len(hook.Entries), len(readText))
			return
		}

		for i, line := range readText {
			entry, err := hook.Entries[i].String()
			if err != nil {
				t.Errorf("Fail to get log entry, %s", err)
				return
			}

			if !reflect.DeepEqual(entry, line) {
				t.Errorf("Read content mismatch \n %s != %s", entry, line)
			}
		}
	}

	logsManager, err := NewManager()
	if err != nil {
		t.Fatal(err)
	}

	defer t.Cleanup(func() {
		if err := logsManager.Cleanup(); err != nil {
			t.Errorf("fail to cleanup logs dir, %s", err)
		}
	})

	logger, err := logsManager.NewLogger(uuid.New().String())
	if err != nil {
		t.Errorf("Fail to create logger")
		return
	}

	hook := test.NewLocal(logger.Entry.Logger)
	testWrite(t, logger)

	for i := 0; i < 1000; i++ {
		t.Run("Test read log", func(t *testing.T) {
			t.Parallel()
			checkReader(t, logger, hook)
		})
	}
}

//TestReadWriteParallel tests the scenario where logs is written and read in parallel
func TestReadWriteParallel(t *testing.T) {
	checkReadWrite := func(t *testing.T, logger *Logger) {
		hook := test.NewLocal(logger.Entry.Logger)
		go testWrite(t, logger)

		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				readText := testRead(ctx, t, logger)

				if len(hook.Entries) != len(readText) {
					t.Errorf("Read content missing %d != %d", len(hook.Entries), len(readText))
					return
				}

				for i, line := range readText {
					entry, err := hook.Entries[i].String()
					if err != nil {
						t.Errorf("Fail to get log entry, %s", err)
						return
					}

					if !reflect.DeepEqual(entry, line) {
						t.Errorf("Read content mismatch \n %s != %s", entry, line)
					}
				}
			}()
		}

		wg.Wait()
	}

	logsManager, err := NewManager()
	if err != nil {
		t.Fatal(err)
	}

	defer t.Cleanup(func() {
		if err := logsManager.Cleanup(); err != nil {
			t.Errorf("fail to cleanup logs dir, %s", err)
		}
	})

	logger, err := logsManager.NewLogger(uuid.New().String())
	if err != nil {
		t.Errorf("Fail to create logger")
		return
	}

	t.Run("Read write parallel", func(t *testing.T) {
		checkReadWrite(t, logger)
	})
}

// testWrite simulates a log writer
func testWrite(t *testing.T, logger *Logger) {
	for i := 0; i < 1000; i++ {
		logger.Entry.Infof(strconv.Itoa(i))
	}

	if err := logger.Close(); err != nil {
		t.Errorf("Fail to close logger %s", err)
	}
}

// testRead simulates a log reader
func testRead(ctx context.Context, t *testing.T, logger *Logger) []string {
	logReader, err := logger.NewReader(ctx)
	if err != nil {
		t.Errorf("Fail to init log reader %s", err)
		return nil
	}

	defer logReader.Close()

	readText := make([]string, 0)
	for {
		line, err := logReader.ReadLine()
		if err != nil {
			break
		}

		readText = append(readText, line)
	}

	return readText
}
