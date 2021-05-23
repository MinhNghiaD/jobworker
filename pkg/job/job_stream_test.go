package job_test

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/MinhNghiaD/jobworker/pkg/job"
	"github.com/MinhNghiaD/jobworker/pkg/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestStreamLog(t *testing.T) {
	manager, err := job.NewManager()
	if err != nil {
		t.Fatal(err)
	}

	defer t.Cleanup(func() {
		if err := manager.Cleanup(); err != nil {
			t.Fatal(err)
		}
	})

	// checker
	checkStream := func(t *testing.T, cmd string, args []string, forceStop bool) {
		jobID, err := manager.CreateJob(cmd, args, "test user")
		if err != nil {
			t.Error(err)
		}

		j, ok := manager.GetJob(jobID)
		if !ok {
			t.Error("Job not found")
		}

		logResults := make([]([]string), 10)

		var wg sync.WaitGroup
		for i := 0; i < len(logResults); i++ {
			wg.Add(1)

			go func(index int) {
				defer wg.Done()
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				logReader, err := j.GetLogReader(ctx)
				if err != nil {
					t.Errorf("Fail to get log reader, err %s", err)
				}
				defer logReader.Close()

				logResults[index] = readLog(t, logReader)
			}(i)
		}

		// Let the jobs run for 10 seconds to collect enough logs
		time.Sleep(10 * time.Second)

		if err = j.Stop(forceStop); err != nil && status.Convert(err).Code() != codes.AlreadyExists {
			t.Error(err)
		}

		wg.Wait()
		checkResults(t, logResults)
	}

	testcases := []struct {
		name      string
		cmd       string
		args      []string
		forceStop bool
	}{
		{
			"Short term command",
			"ls",
			[]string{"-la"},
			false,
		},
		{
			"Short term long result",
			"ps",
			[]string{"-aux"},
			false,
		},
		{
			"long running",
			"top",
			[]string{"-b"},
			false,
		},
		{
			"mask signals force stop",
			"bash",
			[]string{"-c", "trap -- '' SIGINT SIGTERM SIGKILL; while true; do date +%F_%T; sleep 1; done"},
			true,
		},
	}

	for _, testCase := range testcases {
		t.Run(testCase.name, func(t *testing.T) {
			checkStream(t, testCase.cmd, testCase.args, testCase.forceStop)
		})
	}
}

// readLog reads logs from the job
func readLog(t *testing.T, logReader log.Reader) []string {
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

func checkResults(t *testing.T, results []([]string)) {
	if results == nil {
		t.Errorf("No result recorded")
	}

	var wg sync.WaitGroup
	template := results[0]

	for i := 1; i < len(results); i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			if len(template) != len(results[i]) {
				t.Errorf("Results' size are different")
			}

			for index, line := range results[i] {
				if !reflect.DeepEqual(template[index], line) {
					t.Errorf("Results's contents mismatch, %s != %s", template[index], line)
				}
			}
		}(i)
	}

	wg.Wait()
}
