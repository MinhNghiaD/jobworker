package job_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/MinhNghiaD/jobworker/pkg/job"
	"github.com/sirupsen/logrus"
)

func TestStartJobs(t *testing.T) {
	manager, err := job.NewManager()

	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := manager.Cleanup(); err != nil {
			t.Fatal(err)
		}
	}()

	// checker
	checkStartCmd := func(cmd string, args []string) error {
		jobID, err := manager.CreateJob(cmd, args)

		if err != nil {
			return err
		}

		if j, ok := manager.GetJob(jobID); ok {
			j.Stop(false)
		}

		return nil
	}

	// TODO: add more test cases and reinforce the behaviour of the command execution.
	// NOTE: For now, we accept that user can start job with high priviledge. The job will be started but the execution will fail.
	testcases := []struct {
		name      string
		cmd       string
		args      []string
		expectErr bool
	}{
		{
			"Empty Command",
			"",
			[]string{},
			true,
		},
		{
			"Non exist command",
			"abc",
			[]string{},
			true,
		},
		{
			"Short term",
			"ls",
			[]string{"-la"},
			false,
		},
		{
			"File access",
			"mkdir",
			[]string{"/tmp/testdir"},
			false,
		},
		{
			"Check user",
			"whoami",
			[]string{},
			false,
		},
		{
			"long running",
			"top",
			[]string{"-b"},
			false,
		},
		{
			"High priviledge",
			"apt",
			[]string{"update"},
			false,
		},
		{
			"sudo",
			"sudo",
			[]string{"apt", "update"},
			false,
		},
		{
			"bad args",
			"ls",
			[]string{"-wrong"},
			false,
		},
	}

	for _, testCase := range testcases {
		t.Run(testCase.name, func(t *testing.T) {
			err := checkStartCmd(testCase.cmd, testCase.args)

			if (err != nil) != (testCase.expectErr) {
				t.Errorf("Test case %s with input %s returns error %v, while %v error is expected", testCase.name, testCase.cmd, err, testCase.expectErr)
				return
			}
		})
	}
}

func TestGetJobStatus(t *testing.T) {
	manager, err := job.NewManager()

	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := manager.Cleanup(); err != nil {
			t.Fatal(err)
		}
	}()

	// checker
	checkStatusJob := func(cmd string, args []string, expectedStatus *job.ProcStat) error {
		jobID, err := manager.CreateJob(cmd, args)

		if err != nil {
			return err
		}

		time.Sleep(time.Second)

		j, ok := manager.GetJob(jobID)

		if !ok {
			return fmt.Errorf("Job not found")
		}

		defer j.Stop(false)

		time.Sleep(time.Second)

		status := j.Status()

		if (status.Stat != expectedStatus.Stat) || (status.ExitCode != expectedStatus.ExitCode) {
			return fmt.Errorf("Status is %v, when expected %v", status, expectedStatus)
		}

		return err
	}

	testcases := []struct {
		name       string
		cmd        string
		args       []string
		expectStat *job.ProcStat
		expectErr  bool
	}{
		{
			"Empty Command",
			"",
			[]string{},
			&job.ProcStat{
				Stat:     job.STOPPED,
				ExitCode: -1,
			},
			true,
		},
		{
			"Non exist command",
			"abc",
			[]string{},
			&job.ProcStat{
				Stat:     job.STOPPED,
				ExitCode: -1,
			},
			true,
		},
		{
			"Short term command",
			"ls",
			[]string{"-la"},
			&job.ProcStat{
				Stat:     job.EXITED,
				ExitCode: 0,
			},
			false,
		},
		{
			"File access",
			"mkdir",
			[]string{fmt.Sprintf("/tmp/%s", randomString(3))},
			&job.ProcStat{
				Stat:     job.EXITED,
				ExitCode: 0,
			},
			false,
		},
		{
			"User identity",
			"whoami",
			[]string{},
			&job.ProcStat{
				Stat:     job.EXITED,
				ExitCode: 0,
			},
			false,
		},
		{
			"long running",
			"top",
			[]string{"-b"},
			&job.ProcStat{
				Stat:     job.RUNNING,
				ExitCode: -1,
			},
			false,
		},
		{
			"High priviledge",
			"apt",
			[]string{"update"},
			&job.ProcStat{
				Stat:     job.EXITED,
				ExitCode: 100,
			},
			false,
		},
		{
			"sudo",
			"sudo",
			[]string{"apt", "update"},
			&job.ProcStat{
				Stat:     job.EXITED,
				ExitCode: 1,
			},
			false,
		},
		{
			"bad args",
			"ls",
			[]string{"-wrong"},
			&job.ProcStat{
				Stat:     job.EXITED,
				ExitCode: 2,
			},
			false,
		},
	}

	for _, testCase := range testcases {
		t.Run(testCase.name, func(t *testing.T) {
			err := checkStatusJob(testCase.cmd, testCase.args, testCase.expectStat)

			if (err != nil) != (testCase.expectErr) {
				t.Errorf("Test case %s with input %s returns error %v, while %v error is expected", testCase.name, testCase.cmd, err, testCase.expectErr)
				return
			}
		})
	}
}

func TestStopJob(t *testing.T) {
	manager, err := job.NewManager()

	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if err := manager.Cleanup(); err != nil {
			t.Fatal(err)
		}
	}()

	// checker
	checkStopJob := func(cmd string, args []string, force bool, expectedStatus *job.ProcStat) error {
		jobID, err := manager.CreateJob(cmd, args)

		if err != nil {
			return err
		}

		time.Sleep(time.Second)

		j, ok := manager.GetJob(jobID)

		if !ok {
			return fmt.Errorf("Job not found")
		}

		if err = j.Stop(force); err != nil {
			return err
		}

		status := j.Status()

		if (status.Stat != expectedStatus.Stat) || (status.ExitCode != expectedStatus.ExitCode) {
			return fmt.Errorf("Status is %v, when expected %v", status, expectedStatus)
		}

		return err
	}

	testcases := []struct {
		name       string
		cmd        string
		args       []string
		forceStop  bool
		expectStat *job.ProcStat
		expectErr  bool
	}{
		{
			"Empty Command",
			"",
			[]string{},
			false,
			&job.ProcStat{
				Stat:     job.STOPPED,
				ExitCode: -1,
			},
			true,
		},
		{
			"Non exist command",
			"abc",
			[]string{},
			false,
			&job.ProcStat{
				Stat:     job.STOPPED,
				ExitCode: -1,
			},
			true,
		},
		{
			"Sort term command",
			"ls",
			[]string{"-la"},
			false,
			&job.ProcStat{
				Stat:     job.EXITED,
				ExitCode: 0,
			},
			true,
		},
		{
			"File access",
			"mkdir",
			[]string{fmt.Sprintf("/tmp/%s", randomString(3))},
			false,
			&job.ProcStat{
				Stat:     job.EXITED,
				ExitCode: 0,
			},
			true,
		},
		{
			"User identity",
			"whoami",
			[]string{},
			false,
			&job.ProcStat{
				Stat:     job.EXITED,
				ExitCode: 0,
			},
			true,
		},
		{
			"long running",
			"top",
			[]string{"-b"},
			false,
			&job.ProcStat{
				Stat:     job.STOPPED,
				ExitCode: 0,
			},
			false,
		},
		{
			"long running force stop",
			"top",
			[]string{"-b"},
			true,
			&job.ProcStat{
				Stat:     job.STOPPED,
				ExitCode: -1,
			},
			false,
		},
		{
			"High priviledge",
			"apt",
			[]string{"update"},
			false,
			&job.ProcStat{
				Stat:     job.EXITED,
				ExitCode: 100,
			},
			true,
		},
		{
			"sudo",
			"sudo",
			[]string{"apt", "update"},
			false,
			&job.ProcStat{
				Stat:     job.EXITED,
				ExitCode: 1,
			},
			true,
		},
		{
			"bad args",
			"ls",
			[]string{"-wrong"},
			false,
			&job.ProcStat{
				Stat:     job.EXITED,
				ExitCode: 2,
			},
			true,
		},
		{
			"mask signals",
			"bash",
			[]string{"-c", "trap -- '' SIGINT SIGTERM SIGKILL; while true; do date +%F_%T; sleep 1; done"},
			false,
			&job.ProcStat{
				Stat:     job.RUNNING,
				ExitCode: -1,
			},
			true,
		},
		{
			"mask signals force stop",
			"bash",
			[]string{"-c", "trap -- '' SIGINT SIGTERM SIGKILL; while true; do date +%F_%T; sleep 1; done"},
			true,
			&job.ProcStat{
				Stat:     job.STOPPED,
				ExitCode: -1,
			},
			false,
		},
	}

	for _, testCase := range testcases {
		t.Run(testCase.name, func(t *testing.T) {
			err := checkStopJob(testCase.cmd, testCase.args, testCase.forceStop, testCase.expectStat)

			if (err != nil) != (testCase.expectErr) {
				t.Errorf("Test case %s with input %s returns error %v, while %v error is expected", testCase.name, testCase.cmd, err, testCase.expectErr)
				return
			}
		})
	}
}

func TestConcurrence(t *testing.T) {
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
			"High priviledge",
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
			if jobID, err := manager.CreateJob(testCase.cmd, testCase.args); err == nil {
				jobIDs = append(jobIDs, jobID)
			}
		}
	}

	// Simulate client random access
	for i := 0; i < 1000; i++ {
		go func() {
			for j := 0; j < 1000; j++ {
				jobID := jobIDs[rand.Int()%len(jobIDs)]

				time.Sleep(50 * time.Millisecond)

				j, ok := manager.GetJob(jobID)

				if !ok {
					t.Error("Job not found")
					continue
				}

				if err := j.Stop(rand.Int()%2 == 1); err != nil {
					logrus.Infof("Stop %v", err)
				}

				logrus.Infof("status %v", j.Status())
			}
		}()
	}
}

func randomString(length int) string {
	var charset string = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}

	return string(b)
}
