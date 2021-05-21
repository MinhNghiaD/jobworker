package job_test

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/MinhNghiaD/jobworker/pkg/job"
	"github.com/sirupsen/logrus"
)

func TestConcurrency(t *testing.T) {
	manager, err := job.NewManager()

	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := manager.Cleanup(); err != nil {
			t.Fatal(err)
		}
	}()

	testcases := []struct {
		name string
		cmd  string
		args []string
	}{
		{
			"Empty Command",
			"",
			[]string{},
		},
		{
			"Non exist command",
			"abc",
			[]string{},
		},
		{
			"Short term",
			"ls",
			[]string{"-la"},
		},
		{
			"File access",
			"mkdir",
			[]string{"/tmp/testdir"},
		},
		{
			"Check user",
			"whoami",
			[]string{},
		},
		{
			"long running",
			"top",
			[]string{"-b"},
		},
		{
			"High privilege",
			"apt",
			[]string{"update"},
		},
		{
			"sudo",
			"sudo",
			[]string{"apt", "update"},
		},
		{
			"mask signals force stop",
			"bash",
			[]string{"-c", "trap -- '' SIGINT SIGTERM SIGKILL; while true; do date +%F_%T; sleep 1; done"},
		},
	}

	jobIDs := make([]string, 0)

	for i := 0; i < 100; i++ {
		for _, testCase := range testcases {
			if jobID, err := manager.CreateJob(testCase.cmd, testCase.args, "test user"); err == nil {
				jobIDs = append(jobIDs, jobID)
			}
		}
	}

	var wg sync.WaitGroup

	// Simulate client random access
	for i := 0; i < 100; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for j := 0; j < 100; j++ {
				jobID := jobIDs[rand.Int()%len(jobIDs)]

				time.Sleep(50 * time.Millisecond)

				j, ok := manager.GetJob(jobID)

				if !ok {
					t.Error("Job not found")
					continue
				}

				if err := j.Stop(false); err != nil {
					logrus.Infof("Stop %v", err)
				}

				logrus.Infof("status %v", j.Status())
			}
		}()
	}

	wg.Wait()
}
