package job_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/MinhNghiaD/jobworker/api/worker/proto"
	"github.com/MinhNghiaD/jobworker/pkg/job"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestStartJobs tests the creation of a jobs. It tests the scenario when the command can be found and executed or not
func TestStartJobs(t *testing.T) {
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
	checkStartCmd := func(cmd string, args []string) error {
		_, err := manager.CreateJob(cmd, args, "test user")
		if err != nil {
			return err
		}

		return nil
	}

	// TODO: add more test cases and reinforce the behaviour of the command execution.
	// NOTE: For now, we accept that user can start job with high privilege. The job will be started but the execution will fail.
	testcases := []struct {
		name          string
		cmd           string
		args          []string
		expectErrCode codes.Code
	}{
		{
			"Empty Command",
			"",
			[]string{},
			codes.InvalidArgument,
		},
		{
			"Non exist command",
			"abc",
			[]string{},
			codes.InvalidArgument,
		},
		{
			"Short term",
			"ls",
			[]string{"-la"},
			codes.OK,
		},
		{
			"File access",
			"mkdir",
			[]string{fmt.Sprintf("/tmp/%s", randomString(3))},
			codes.OK,
		},
		{
			"Check user",
			"whoami",
			[]string{},
			codes.OK,
		},
		{
			"long running",
			"top",
			[]string{"-b"},
			codes.OK,
		},
		{
			"High privilege",
			"apt",
			[]string{"update"},
			codes.OK,
		},
		{
			"bad args",
			"ls",
			[]string{"-wrong"},
			codes.OK,
		},
	}

	for _, testCase := range testcases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			err := checkStartCmd(testCase.cmd, testCase.args)

			if err != nil {
				s := status.Convert(err)
				if s.Code() != testCase.expectErrCode {
					t.Errorf("Test case %s with input %s returns error %v, while %v error is expected", testCase.name, testCase.cmd, s.Code(), testCase.expectErrCode)
				}
			} else if testCase.expectErrCode != codes.OK {
				t.Errorf("Test case %s with input %s returns no error, while %v error is expected", testCase.name, testCase.cmd, testCase.expectErrCode)
			}
		})
	}
}

// TestGetShortJobStatus tests status verification of short-running command. It will polling until the job exited normally and examined it status.
// The test fails when the job status of an exited job is not corresponding to the prediction.
func TestGetShortJobStatus(t *testing.T) {
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
	checkStatusJob := func(cmd string, args []string, expectedStatus *proto.ProcessStatus) error {
		jobID, err := manager.CreateJob(cmd, args, "test user")
		if err != nil {
			return err
		}

		var st *proto.JobStatus

		for i := 1; i < 100; i++ {
			time.Sleep(10 * time.Millisecond)
			j, ok := manager.GetJob(jobID)
			if !ok {
				return status.Errorf(codes.NotFound, "Job not found")
			}

			st = j.Status()
			if st.Status.State != proto.ProcessState_RUNNING {
				break
			}
		}

		if st == nil || (st.Status.State != expectedStatus.State) || (st.Status.ExitCode != expectedStatus.ExitCode) {
			t.Errorf("Status is %v, when expected %v", st, expectedStatus)
		}

		return nil
	}

	testcases := []struct {
		name          string
		cmd           string
		args          []string
		expectStat    *proto.ProcessStatus
		expectErrCode codes.Code
	}{
		{
			"Empty Command",
			"",
			[]string{},
			&proto.ProcessStatus{
				State:    proto.ProcessState_STOPPED,
				ExitCode: -1,
			},
			codes.InvalidArgument,
		},
		{
			"Non exist command",
			"abc",
			[]string{},
			&proto.ProcessStatus{
				State:    proto.ProcessState_STOPPED,
				ExitCode: -1,
			},
			codes.InvalidArgument,
		},
		{
			"Short term command",
			"ls",
			[]string{"-la"},
			&proto.ProcessStatus{
				State:    proto.ProcessState_EXITED,
				ExitCode: 0,
			},
			codes.OK,
		},
		{
			"File access",
			"mkdir",
			[]string{fmt.Sprintf("/tmp/%s", randomString(3))},
			&proto.ProcessStatus{
				State:    proto.ProcessState_EXITED,
				ExitCode: 0,
			},
			codes.OK,
		},
		{
			"User identity",
			"whoami",
			[]string{},
			&proto.ProcessStatus{
				State:    proto.ProcessState_EXITED,
				ExitCode: 0,
			},
			codes.OK,
		},
		{
			"High privilege",
			"apt",
			[]string{"update"},
			&proto.ProcessStatus{
				State:    proto.ProcessState_EXITED,
				ExitCode: 100,
			},
			codes.OK,
		},
		{
			"bad args",
			"ls",
			[]string{"-wrong"},
			&proto.ProcessStatus{
				State:    proto.ProcessState_EXITED,
				ExitCode: 2,
			},
			codes.OK,
		},
	}

	for _, testCase := range testcases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			err := checkStatusJob(testCase.cmd, testCase.args, testCase.expectStat)

			if err != nil {
				s := status.Convert(err)
				if s.Code() != testCase.expectErrCode {
					t.Errorf("Test case %s with input %s returns error %v, while %v error is expected", testCase.name, testCase.cmd, s.Code(), testCase.expectErrCode)
				}
			} else if testCase.expectErrCode != codes.OK {
				t.Errorf("Test case %s with input %s returns no error, while %v error is expected", testCase.name, testCase.cmd, testCase.expectErrCode)
			}
		})
	}
}

// TestGetLongJobStatus tests status verification of long-running command. It will polling to see if the jon is still running.
// The test fails when a job is not running when it was predicted
func TestGetLongJobStatus(t *testing.T) {
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
	checkStatusJob := func(cmd string, args []string, expectedStatus *proto.ProcessStatus) error {
		jobID, err := manager.CreateJob(cmd, args, "test user")
		if err != nil {
			return err
		}

		j, ok := manager.GetJob(jobID)
		if !ok {
			return status.Errorf(codes.NotFound, "Job not found")
		}

		for i := 1; i < 100; i++ {
			time.Sleep(10 * time.Millisecond)
			st := j.Status()
			if (st.Status.State != expectedStatus.State) || (st.Status.ExitCode != expectedStatus.ExitCode) {
				t.Errorf("Status is %v, when expected %v", st, expectedStatus)
			}
		}

		return nil
	}

	testcases := []struct {
		name          string
		cmd           string
		args          []string
		expectStat    *proto.ProcessStatus
		expectErrCode codes.Code
	}{
		{
			"long running",
			"top",
			[]string{"-b"},
			&proto.ProcessStatus{
				State:    proto.ProcessState_RUNNING,
				ExitCode: -1,
			},
			codes.OK,
		},
		{
			"tail f",
			"tail",
			[]string{"-f", "/dev/null"},
			&proto.ProcessStatus{
				State:    proto.ProcessState_RUNNING,
				ExitCode: -1,
			},
			codes.OK,
		},
		{
			"mask signals",
			"bash",
			[]string{"-c", "trap -- '' SIGINT SIGTERM SIGKILL; while true; do date +%F_%T; sleep 1; done"},
			&proto.ProcessStatus{
				State:    proto.ProcessState_RUNNING,
				ExitCode: -1,
			},
			codes.OK,
		},
	}

	for _, testCase := range testcases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			err := checkStatusJob(testCase.cmd, testCase.args, testCase.expectStat)

			if err != nil {
				s := status.Convert(err)
				if s.Code() != testCase.expectErrCode {
					t.Errorf("Test case %s with input %s returns error %v, while %v error is expected", testCase.name, testCase.cmd, s.Code(), testCase.expectErrCode)
				}
			} else if testCase.expectErrCode != codes.OK {
				t.Errorf("Test case %s with input %s returns no error, while %v error is expected", testCase.name, testCase.cmd, testCase.expectErrCode)
			}
		})
	}
}

// TestStopJob tests the request to stop a job and their exit status. The test will verify the error code returned by the request,
// as well as the exit status of the job
func TestStopJob(t *testing.T) {
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
	checkStopJob := func(cmd string, args []string, force bool, expectedStatus *proto.ProcessStatus) error {
		jobID, err := manager.CreateJob(cmd, args, "test user")
		if err != nil {
			return err
		}

		time.Sleep(time.Second)

		j, ok := manager.GetJob(jobID)
		if !ok {
			return status.Errorf(codes.NotFound, "Job not found")
		}

		if err = j.Stop(force); err != nil {
			return err
		}

		status := j.Status()
		if (status.Status.State != expectedStatus.State) || (status.Status.ExitCode != expectedStatus.ExitCode) {
			t.Errorf("Status is %v, when expected %v", status, expectedStatus)
		}

		return nil
	}

	testcases := []struct {
		name          string
		cmd           string
		args          []string
		forceStop     bool
		expectStat    *proto.ProcessStatus
		expectErrCode codes.Code
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
			codes.InvalidArgument,
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
			codes.InvalidArgument,
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
			codes.AlreadyExists,
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
			codes.AlreadyExists,
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
			codes.AlreadyExists,
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
			codes.OK,
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
			codes.OK,
		},
		{
			"High privilege",
			"apt",
			[]string{"update"},
			false,
			&proto.ProcessStatus{
				State:    proto.ProcessState_EXITED,
				ExitCode: 100,
			},
			codes.AlreadyExists,
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
			codes.AlreadyExists,
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
			codes.DeadlineExceeded,
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
			codes.OK,
		},
	}

	for _, testCase := range testcases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			err := checkStopJob(testCase.cmd, testCase.args, testCase.forceStop, testCase.expectStat)

			if err != nil {
				s := status.Convert(err)
				if s.Code() != testCase.expectErrCode {
					t.Errorf("Test case %s with input %s returns error %v, while %v error is expected", testCase.name, testCase.cmd, s.Code(), testCase.expectErrCode)
				}
			} else if testCase.expectErrCode != codes.OK {
				t.Errorf("Test case %s with input %s returns no error, while %v error is expected", testCase.name, testCase.cmd, testCase.expectErrCode)
			}
		})
	}
}

// randomString generate a random string contains only alphabet-numerica characters
func randomString(length int) string {
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)

	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}

	return string(b)
}
