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

		job, err := client.Stub.StartJob(context.Background(), command)
		if err != nil {
			return err
		}

		logrus.Infof("Started job %s", job)

		return nil
	}

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
			"High privilege",
			"apt",
			[]string{"update"},
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
			err := checkStartJob(testCase.cmd, testCase.args)

			if (err != nil) != (testCase.expectErr) {
				t.Errorf("Test case %s with input %s returns error %v, while %v error is expected", testCase.name, testCase.cmd, err, testCase.expectErr)
				return
			}
		})
	}
}

func TestGetJobStatus(t *testing.T) {
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

		job, err := client.Stub.StartJob(context.Background(), command)
		if err != nil {
			return err
		}

		logrus.Infof("Started job %s", job)

		time.Sleep(time.Second)

		jobStatus, err := client.Stub.QueryJob(context.Background(), job)
		if err != nil {
			return err
		}

		logrus.Infof("Job status %s", jobStatus)

		if (jobStatus.Status.State != expectedStatus.State) || (jobStatus.Status.ExitCode != expectedStatus.ExitCode) {
			return fmt.Errorf("Status is %v, when expected %v", jobStatus, expectedStatus)
		}

		return nil
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
			"High privilege",
			"apt",
			[]string{"update"},
			&proto.ProcessStatus{
				State:    proto.ProcessState_EXITED,
				ExitCode: 100,
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
			err := checkQueryJob(testCase.cmd, testCase.args, testCase.expectStat)

			if (err != nil) != (testCase.expectErr) {
				t.Errorf("Test case %s with input %s returns error %v, while %v error is expected", testCase.name, testCase.cmd, err, testCase.expectErr)
				return
			}
		})
	}
}

func TestStopJob(t *testing.T) {
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

		job, err := client.Stub.StartJob(context.Background(), command)
		if err != nil {
			return err
		}

		logrus.Infof("Started job %s", job)

		time.Sleep(time.Second)

		jobStatus, err := client.Stub.StopJob(context.Background(), &proto.StopRequest{Job: job, Force: force})
		if err != nil {
			return err
		}

		logrus.Infof("Job status %s", jobStatus)

		if (jobStatus.Status.State != expectedStatus.State) || (jobStatus.Status.ExitCode != expectedStatus.ExitCode) {
			return fmt.Errorf("Status is %v, when expected %v", jobStatus, expectedStatus)
		}

		return nil
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
			"High privilege",
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

	// Start a job
	command := &proto.Command{
		Cmd:  "top",
		Args: []string{"-b"},
	}

	job, err := client.Stub.StartJob(context.Background(), command)
	if err != nil {
		t.Error(err)
	}

	jobStatus, err := client.Stub.QueryJob(context.Background(), job)
	if err != nil {
		t.Error(err)
	}

	logrus.Infof("Job status %s", jobStatus)

	if (jobStatus.Status.State != proto.ProcessState_RUNNING) || (jobStatus.Status.ExitCode != -1) {
		t.Error("Status is not as expected")
	}

	// Query and stop with wrong id
	_, err = client.Stub.QueryJob(context.Background(), &proto.Job{Id: uuid.New().String()})
	if err == nil {
		t.Error("Reported no error when error is expected")
	} else {
		if st, ok := status.FromError(err); !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("Wrong error reported %s", err)
		}
	}

	_, err = client.Stub.StopJob(context.Background(), &proto.StopRequest{Job: &proto.Job{Id: uuid.New().String()}, Force: true})
	if err == nil {
		t.Error("Reported no error when error is expected")
	} else {
		if st, ok := status.FromError(err); !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("Wrong error reported %s", err)
		}
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
