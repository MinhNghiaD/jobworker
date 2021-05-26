package service_test

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"sync"
	"testing"
	"time"

	pb "google.golang.org/protobuf/proto"

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
	server, cli := initTestServerClient(t)
	if server == nil || cli == nil {
		t.FailNow()
	}

	go server.Serve()
	defer t.Cleanup(func() {
		server.Close()
		cli.Close()
	})

	// checker
	checkStartJob := func(cmd string, args []string) error {
		command := &proto.Command{
			Cmd:  cmd,
			Args: args,
		}

		_, err := cli.StartJob(context.Background(), command)
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
	server, cli := initTestServerClient(t)
	if server == nil || cli == nil {
		t.FailNow()
	}

	go server.Serve()
	defer t.Cleanup(func() {
		server.Close()
		cli.Close()
	})

	// checker
	checkQueryJob := func(cmd string, args []string, expectedStatus *proto.ProcessStatus) error {
		command := &proto.Command{
			Cmd:  cmd,
			Args: args,
		}

		job, err := cli.StartJob(context.Background(), command)
		if err != nil {
			return err
		}

		logrus.Infof("Started job %s", job)

		var jobStatus *proto.JobStatus
		for i := 0; i < 100; i++ {
			time.Sleep(10 * time.Millisecond)
			jobStatus, err = cli.QueryJob(context.Background(), job)
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
	server, cli := initTestServerClient(t)
	if server == nil || cli == nil {
		t.FailNow()
	}

	go server.Serve()
	defer t.Cleanup(func() {
		server.Close()
		cli.Close()
	})

	// checker
	checkQueryJob := func(cmd string, args []string, expectedStatus *proto.ProcessStatus) error {
		command := &proto.Command{
			Cmd:  cmd,
			Args: args,
		}

		job, err := cli.StartJob(context.Background(), command)
		if err != nil {
			return err
		}

		logrus.Infof("Started job %s", job)

		for i := 1; i < 100; i++ {
			time.Sleep(10 * time.Millisecond)
			jobStatus, err := cli.QueryJob(context.Background(), job)
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
	server, cli := initTestServerClient(t)
	if server == nil || cli == nil {
		t.FailNow()
	}

	go server.Serve()
	defer t.Cleanup(func() {
		server.Close()
		cli.Close()
	})

	// checker
	checkStopJob := func(cmd string, args []string, force bool, expectedStatus *proto.ProcessStatus) error {
		command := &proto.Command{
			Cmd:  cmd,
			Args: args,
		}

		job, err := cli.StartJob(context.Background(), command)
		if err != nil {
			return err
		}

		logrus.Infof("Started job %s", job)
		time.Sleep(time.Second)

		jobStatus, err := cli.StopJob(context.Background(), &proto.StopRequest{Job: job, Force: force})
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
	server, cli := initTestServerClient(t)
	if server == nil || cli == nil {
		t.FailNow()
	}

	go server.Serve()
	defer t.Cleanup(func() {
		server.Close()
		cli.Close()
	})

	t.Run("Normal execution", func(t *testing.T) {
		t.Parallel()
		command := &proto.Command{
			Cmd:  "top",
			Args: []string{"-b"},
		}

		job, err := cli.StartJob(context.Background(), command)
		if err != nil {
			t.Error(err)
		}

		jobStatus, err := cli.QueryJob(context.Background(), job)
		if err != nil {
			t.Error(err)
		}

		if (jobStatus.Status.State != proto.ProcessState_RUNNING) || (jobStatus.Status.ExitCode != -1) {
			t.Error("Status is not as expected")
		}
	})

	t.Run("Query wrong id", func(t *testing.T) {
		t.Parallel()
		_, err := cli.QueryJob(context.Background(), &proto.Job{Id: uuid.New().String()})

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
		_, err := cli.StopJob(context.Background(), &proto.StopRequest{Job: &proto.Job{Id: uuid.New().String()}, Force: false})

		if err == nil {
			t.Error("Reported no error when error is expected")
		} else {
			s := status.Convert(err)
			if s.Code() != codes.NotFound {
				t.Errorf("Wrong error reported %s, expected %s", s.Code(), codes.NotFound)
			}
		}
	})

	t.Run("Stream wrong id", func(t *testing.T) {
		t.Parallel()
		receiver, err := cli.GetLogReceiver(context.Background(), &proto.Job{Id: uuid.New().String()})
		for err == nil {
			_, err = receiver.Read()
		}

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

// TestStreaming tests real-time log streaming, where a long running job is running in the background,
// and there are multiple clients request to receive its log.
// The test fails when clients fail to receive the logs or when the logs received by the clients are not the same.
func TestStreaming(t *testing.T) {
	server, cli := initTestServerClient(t)
	if server == nil || cli == nil {
		t.FailNow()
	}

	go server.Serve()
	defer t.Cleanup(func() {
		server.Close()
		cli.Close()
	})

	// checker
	checkStream := func(t *testing.T, cmd string, args []string, forceStop bool) {
		command := &proto.Command{
			Cmd:  cmd,
			Args: args,
		}

		j, err := cli.StartJob(context.Background(), command)
		if err != nil {
			t.Error(err)
		}

		logResults := make([]([]*proto.Log), 10)
		var wg sync.WaitGroup

		for i := 0; i < len(logResults); i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				receiver, err := cli.GetLogReceiver(ctx, j)
				if err != nil {
					t.Error(err)
				}

				logResults[index] = readLog(t, receiver)
			}(i)
		}

		// Let the jobs run for 5 seconds
		time.Sleep(5 * time.Second)
		cli.StopJob(context.Background(), &proto.StopRequest{Job: j, Force: forceStop})
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

// initTestServerClient creates test server and test client.
// If the call is success, it will return the server and client for testing.
// If it fail, neither of them is returned
func initTestServerClient(t *testing.T) (*service.WorkerServer, *client.Client) {
	server, err := service.NewServer(7777)
	if err != nil {
		t.Fatal(err)
	}

	cli, err := client.New("127.0.0.1:7777")
	if err != nil {
		t.Fatalf("Fail to init client %s", err)
	}

	return server, cli
}

// readLog reads logs from the job
func readLog(t *testing.T, receiver *client.LogReceiver) []*proto.Log {
	readText := make([]*proto.Log, 0)
	for {
		line, err := receiver.Read()
		if err != nil {
			if err != io.EOF {
				t.Error(err)
			}
			break
		}

		readText = append(readText, line)
	}

	return readText
}

// checkResults verify if the logs received by different stream is the same
func checkResults(t *testing.T, results []([]*proto.Log)) {
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
				if !pb.Equal(template[index], line) {
					t.Errorf("Results's contents mismatch: \n %s != %s", template[index], line)
				}
			}
		}(i)
	}

	wg.Wait()
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
