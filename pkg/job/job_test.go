package job_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/MinhNghiaD/jobworker/api/worker/proto"
	"github.com/MinhNghiaD/jobworker/pkg/job"
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
		jobID, err := manager.CreateJob(cmd, args, "test user")

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
	checkStatusJob := func(cmd string, args []string, expectedStatus *proto.ProcessStatus) error {
		jobID, err := manager.CreateJob(cmd, args, "test user")

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

		if (status.Status.State != expectedStatus.State) || (status.Status.ExitCode != expectedStatus.ExitCode) {
			return fmt.Errorf("Status is %v, when expected %v", status, expectedStatus)
		}

		return err
	}

	testcases := []struct {
		name       string
		cmd        string
		args       []string
		expectStat *proto.ProcessStatus
		expectErr  bool
	}{
		{
			"Empty Command",
			"",
			[]string{},
			&proto.ProcessStatus{
				State:    proto.ProcessState_STOPPED,
				ExitCode: -1,
			},
			true,
		},
		{
			"Non exist command",
			"abc",
			[]string{},
			&proto.ProcessStatus{
				State:    proto.ProcessState_STOPPED,
				ExitCode: -1,
			},
			true,
		},
		{
			"Short term command",
			"ls",
			[]string{"-la"},
			&proto.ProcessStatus{
				State:    proto.ProcessState_EXITED,
				ExitCode: 0,
			},
			false,
		},
		{
			"File access",
			"mkdir",
			[]string{fmt.Sprintf("/tmp/%s", randomString(3))},
			&proto.ProcessStatus{
				State:    proto.ProcessState_EXITED,
				ExitCode: 0,
			},
			false,
		},
		{
			"User identity",
			"whoami",
			[]string{},
			&proto.ProcessStatus{
				State:    proto.ProcessState_EXITED,
				ExitCode: 0,
			},
			false,
		},
		{
			"long running",
			"top",
			[]string{"-b"},
			&proto.ProcessStatus{
				State:    proto.ProcessState_RUNNING,
				ExitCode: -1,
			},
			false,
		},
		{
			"High priviledge",
			"apt",
			[]string{"update"},
			&proto.ProcessStatus{
				State:    proto.ProcessState_EXITED,
				ExitCode: 100,
			},
			false,
		},
		{
			"sudo",
			"sudo",
			[]string{"apt", "update"},
			&proto.ProcessStatus{
				State:    proto.ProcessState_EXITED,
				ExitCode: 1,
			},
			false,
		},
		{
			"bad args",
			"ls",
			[]string{"-wrong"},
			&proto.ProcessStatus{
				State:    proto.ProcessState_EXITED,
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
	checkStopJob := func(cmd string, args []string, force bool, expectedStatus *proto.ProcessStatus) error {
		jobID, err := manager.CreateJob(cmd, args, "test user")

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

		if (status.Status.State != expectedStatus.State) || (status.Status.ExitCode != expectedStatus.ExitCode) {
			return fmt.Errorf("Status is %v, when expected %v", status, expectedStatus)
		}

		return err
	}

	testcases := []struct {
		name       string
		cmd        string
		args       []string
		forceStop  bool
		expectStat *proto.ProcessStatus
		expectErr  bool
	}{
		{
			"Empty Command",
			"",
			[]string{},
			false,
			&proto.ProcessStatus{
				State:    proto.ProcessState_STOPPED,
				ExitCode: -1,
			},
			true,
		},
		{
			"Non exist command",
			"abc",
			[]string{},
			false,
			&proto.ProcessStatus{
				State:    proto.ProcessState_STOPPED,
				ExitCode: -1,
			},
			true,
		},
		{
			"Sort term command",
			"ls",
			[]string{"-la"},
			false,
			&proto.ProcessStatus{
				State:    proto.ProcessState_EXITED,
				ExitCode: 0,
			},
			true,
		},
		{
			"File access",
			"mkdir",
			[]string{fmt.Sprintf("/tmp/%s", randomString(3))},
			false,
			&proto.ProcessStatus{
				State:    proto.ProcessState_EXITED,
				ExitCode: 0,
			},
			true,
		},
		{
			"User identity",
			"whoami",
			[]string{},
			false,
			&proto.ProcessStatus{
				State:    proto.ProcessState_EXITED,
				ExitCode: 0,
			},
			true,
		},
		{
			"long running",
			"top",
			[]string{"-b"},
			false,
			&proto.ProcessStatus{
				State:    proto.ProcessState_STOPPED,
				ExitCode: 0,
			},
			false,
		},
		{
			"long running force stop",
			"top",
			[]string{"-b"},
			true,
			&proto.ProcessStatus{
				State:    proto.ProcessState_STOPPED,
				ExitCode: -1,
			},
			false,
		},
		{
			"High priviledge",
			"apt",
			[]string{"update"},
			false,
			&proto.ProcessStatus{
				State:    proto.ProcessState_EXITED,
				ExitCode: 100,
			},
			true,
		},
		{
			"sudo",
			"sudo",
			[]string{"apt", "update"},
			false,
			&proto.ProcessStatus{
				State:    proto.ProcessState_EXITED,
				ExitCode: 1,
			},
			true,
		},
		{
			"bad args",
			"ls",
			[]string{"-wrong"},
			false,
			&proto.ProcessStatus{
				State:    proto.ProcessState_EXITED,
				ExitCode: 2,
			},
			true,
		},
		{
			"mask signals",
			"bash",
			[]string{"-c", "trap -- '' SIGINT SIGTERM SIGKILL; while true; do date +%F_%T; sleep 1; done"},
			false,
			&proto.ProcessStatus{
				State:    proto.ProcessState_RUNNING,
				ExitCode: -1,
			},
			true,
		},
		{
			"mask signals force stop",
			"bash",
			[]string{"-c", "trap -- '' SIGINT SIGTERM SIGKILL; while true; do date +%F_%T; sleep 1; done"},
			true,
			&proto.ProcessStatus{
				State:    proto.ProcessState_STOPPED,
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

func randomString(length int) string {
	var charset string = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}

	return string(b)
}
