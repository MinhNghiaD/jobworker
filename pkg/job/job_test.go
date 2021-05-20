package job_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/MinhNghiaD/jobworker/pkg/job"
)

func TestStartJobs(t *testing.T) {
	manager, err := job.NewManager()

	if err != nil {
		t.Fatal(err)
	}

	// checker
	checkStartCmd := func(cmd string, args []string) error {
		_, err := manager.CreateJob(cmd, args)

		return err
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
			"ls",
			"ls",
			[]string{"-la"},
			false,
		},
		{
			"mkdir",
			"mkdir",
			[]string{"/tmp/testdir"},
			false,
		},
		{
			"whoami",
			"whoami",
			[]string{},
			false,
		},
		{
			"top",
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

	// checker
	checkStatusJob := func(cmd string, args []string, expectedStatus *job.ProcStat) error {
		j, err := manager.CreateJob(cmd, args)

		if err != nil {
			return err
		}

		time.Sleep(time.Second)

		status := j.Status()

		if (status.Stat != expectedStatus.Stat) || (status.ExitCode != expectedStatus.ExitCode) {
			return fmt.Errorf("Status is %v, when expected %v", status, expectedStatus)
		}

		return err
	}

	// TODO: add more test cases and reinforce the behaviour of the command execution.
	// NOTE: For now, we accept that user can start job with high priviledge. The job will be started but the execution will fail.
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
			"ls",
			"ls",
			[]string{"-la"},
			&job.ProcStat{
				Stat:     job.EXITED,
				ExitCode: 0,
			},
			false,
		},
		{
			"mkdir",
			"mkdir",
			[]string{fmt.Sprintf("/tmp/%s", randomString(3))},
			&job.ProcStat{
				Stat:     job.EXITED,
				ExitCode: 0,
			},
			false,
		},
		{
			"whoami",
			"whoami",
			[]string{},
			&job.ProcStat{
				Stat:     job.EXITED,
				ExitCode: 0,
			},
			false,
		},
		{
			"top",
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

func randomString(length int) string {
	var charset string = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}

	return string(b)
}