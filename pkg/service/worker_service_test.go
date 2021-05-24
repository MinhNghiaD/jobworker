package service_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/MinhNghiaD/jobworker/api/client"
	"github.com/MinhNghiaD/jobworker/api/worker/proto"
	"github.com/MinhNghiaD/jobworker/pkg/service"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NOTE we won't use context timeout for grpc testing because the respond time in test is not fast enough

// TestStartJobs tests the creation of a jobs via grpc unary request
func TestStartJobs(t *testing.T) {
	server, err := service.NewServer(7777)
	if err != nil {
		t.Fatal(err)
	}

	go server.Serve()

	client, err := client.New("127.0.0.1:7777")
	if err != nil {
		t.Fatalf("Fail to init client %s", err)
	}

	defer t.Cleanup(func() {
		server.Close()
		client.Close()
	})

	// checker
	checkStartJob := func(cmd string, args []string) error {
		command := &proto.Command{
			Cmd:  cmd,
			Args: args,
		}

		_, err := client.StartJob(context.Background(), command)
		if err != nil {
			return err
		}

		return nil
	}

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
			err := checkStartJob(testCase.cmd, testCase.args)

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

// TestQueryShortJob tests status verification of short-running command. It will polling until the job exited normally and examined it status.
// The test fails when the job status of an exited job is not corresponding to the prediction.
func TestQueryShortJob(t *testing.T) {
	server, err := service.NewServer(7777)
	if err != nil {
		t.Fatal(err)
	}

	go server.Serve()

	client, err := client.New("127.0.0.1:7777")
	if err != nil {
		t.Fatalf("Fail to init client %s", err)
	}

	defer t.Cleanup(func() {
		server.Close()
		client.Close()
	})

	// checker
	checkQueryJob := func(cmd string, args []string, expectedStatus *proto.ProcessStatus) error {
		command := &proto.Command{
			Cmd:  cmd,
			Args: args,
		}

		job, err := client.StartJob(context.Background(), command)
		if err != nil {
			return err
		}

		logrus.Infof("Started job %s", job)

		var jobStatus *proto.JobStatus
		for i := 0; i < 100; i++ {
			time.Sleep(10 * time.Millisecond)
			jobStatus, err = client.QueryJob(context.Background(), job)
			if err != nil {
				return err
			}

			if jobStatus.Status.State != proto.ProcessState_RUNNING {
				break
			}
		}

		if (jobStatus.Status.State != expectedStatus.State) || (jobStatus.Status.ExitCode != expectedStatus.ExitCode) {
			t.Errorf("Status is %v, when expected %v", jobStatus, expectedStatus)
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
			err := checkQueryJob(testCase.cmd, testCase.args, testCase.expectStat)

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

// TestQueryLongJob tests status verification of long-running command. It will polling to see if the jon is still running.
// The test fails when a job is not running when it was predicted
func TestQueryLongJob(t *testing.T) {
	server, err := service.NewServer(7777)
	if err != nil {
		t.Fatal(err)
	}

	go server.Serve()

	client, err := client.New("127.0.0.1:7777")
	if err != nil {
		t.Fatalf("Fail to init client %s", err)
	}

	defer t.Cleanup(func() {
		server.Close()
		client.Close()
	})

	// checker
	checkQueryJob := func(cmd string, args []string, expectedStatus *proto.ProcessStatus) error {
		command := &proto.Command{
			Cmd:  cmd,
			Args: args,
		}

		job, err := client.StartJob(context.Background(), command)
		if err != nil {
			return err
		}

		logrus.Infof("Started job %s", job)

		for i := 1; i < 100; i++ {
			time.Sleep(10 * time.Millisecond)
			jobStatus, err := client.QueryJob(context.Background(), job)
			if err != nil {
				return err
			}

			if (jobStatus.Status.State != expectedStatus.State) || (jobStatus.Status.ExitCode != expectedStatus.ExitCode) {
				t.Errorf("Status is %v, when expected %v", jobStatus, expectedStatus)
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
			err := checkQueryJob(testCase.cmd, testCase.args, testCase.expectStat)

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

// TestStopJobs tests the request to stop a job and query their exit status via grpc unary request.
// The test will verify the error code returned by the request, as well as the exit status of the job
func TestStopJobs(t *testing.T) {
	server, err := service.NewServer(7777)
	if err != nil {
		t.Fatal(err)
	}

	go server.Serve()

	client, err := client.New("127.0.0.1:7777")
	if err != nil {
		t.Fatalf("Fail to init client %s", err)
	}

	defer t.Cleanup(func() {
		server.Close()
		client.Close()
	})

	// checker
	checkStopJob := func(cmd string, args []string, force bool, expectedStatus *proto.ProcessStatus) error {
		command := &proto.Command{
			Cmd:  cmd,
			Args: args,
		}

		job, err := client.StartJob(context.Background(), command)
		if err != nil {
			return err
		}

		logrus.Infof("Started job %s", job)
		time.Sleep(time.Second)

		jobStatus, err := client.StopJob(context.Background(), &proto.StopRequest{Job: job, Force: force})
		if err != nil {
			return err
		}

		if (jobStatus.Status.State != expectedStatus.State) || (jobStatus.Status.ExitCode != expectedStatus.ExitCode) {
			t.Errorf("Status is %v, when expected %v", jobStatus, expectedStatus)
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
			"Short term command",
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

// TestRequestBadJobs tests some of common unhappy cases, where the requests have bad arguments to be processed
func TestRequestBadJobs(t *testing.T) {
	server, err := service.NewServer(7777)
	if err != nil {
		t.Fatal(err)
	}

	go server.Serve()

	client, err := client.New("127.0.0.1:7777")
	if err != nil {
		t.Fatalf("Fail to init client %s", err)
	}

	defer t.Cleanup(func() {
		server.Close()
		client.Close()
	})

	t.Run("Normal execution", func(t *testing.T) {
		t.Parallel()
		command := &proto.Command{
			Cmd:  "top",
			Args: []string{"-b"},
		}

		job, err := client.StartJob(context.Background(), command)
		if err != nil {
			t.Error(err)
		}

		jobStatus, err := client.QueryJob(context.Background(), job)
		if err != nil {
			t.Error(err)
		}

		if (jobStatus.Status.State != proto.ProcessState_RUNNING) || (jobStatus.Status.ExitCode != -1) {
			t.Error("Status is not as expected")
		}
	})

	t.Run("Query wrong id", func(t *testing.T) {
		t.Parallel()
		_, err := client.QueryJob(context.Background(), &proto.Job{Id: uuid.New().String()})

		if err == nil {
			t.Error("Reported no error when error is expected")
		} else {
			s := status.Convert(err)
			if s.Code() != codes.NotFound {
				t.Errorf("Wrong error reported %s, expected %s", s.Code(), codes.NotFound)
			}
		}
	})

	t.Run("Stop wrong id", func(t *testing.T) {
		t.Parallel()
		_, err := client.StopJob(context.Background(), &proto.StopRequest{Job: &proto.Job{Id: uuid.New().String()}, Force: false})

		if err == nil {
			t.Error("Reported no error when error is expected")
		} else {
			s := status.Convert(err)
			if s.Code() != codes.NotFound {
				t.Errorf("Wrong error reported %s, expected %s", s.Code(), codes.NotFound)
			}
		}
	})
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
