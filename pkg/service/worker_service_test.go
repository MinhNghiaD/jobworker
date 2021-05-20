package service_test

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/MinhNghiaD/jobworker/api/worker/proto"
	"github.com/MinhNghiaD/jobworker/pkg/service"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

func TestStartJobs(t *testing.T) {
	server, listener := newTestServer()

	go server.Serve(listener)
	defer listener.Close()

	clientConn := newTestClientConn()
	defer clientConn.Close()
	client := proto.NewWorkerServiceClient(clientConn)

	// checker
	checkStartJob := func(cmd string, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		command := &proto.Command{
			Cmd:  cmd,
			Args: args,
		}

		job, err := client.StartJob(ctx, command)

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
			err := checkStartJob(testCase.cmd, testCase.args)

			if (err != nil) != (testCase.expectErr) {
				t.Errorf("Test case %s with input %s returns error %v, while %v error is expected", testCase.name, testCase.cmd, err, testCase.expectErr)
				return
			}
		})
	}
}

func TestGetJobStatus(t *testing.T) {
	server, listener := newTestServer()

	go server.Serve(listener)
	defer listener.Close()

	clientConn := newTestClientConn()
	defer clientConn.Close()
	client := proto.NewWorkerServiceClient(clientConn)

	// checker
	checkQueryJob := func(cmd string, args []string, expectedStatus *proto.ProcessStatus) error {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)

		command := &proto.Command{
			Cmd:  cmd,
			Args: args,
		}

		job, err := client.StartJob(ctx, command)

		if err != nil {
			return err
		}

		logrus.Infof("Started job %s", job)

		time.Sleep(time.Second)

		ctx, cancel = context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		status, err := client.QueryJob(ctx, job)

		if err != nil {
			return err
		}

		logrus.Infof("Job status %s", status)

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
			err := checkQueryJob(testCase.cmd, testCase.args, testCase.expectStat)

			if (err != nil) != (testCase.expectErr) {
				t.Errorf("Test case %s with input %s returns error %v, while %v error is expected", testCase.name, testCase.cmd, err, testCase.expectErr)
				return
			}
		})
	}
}

func newTestServer() (*grpc.Server, net.Listener) {
	logrus.Infoln("Listen at port 7777")
	listener, err := net.Listen("tcp", "0.0.0.0:7777")

	if err != nil {
		logrus.Fatalf("failed to listen: %v", err)
	}

	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)

	service, err := service.NewWorkerService()

	if err != nil {
		logrus.Fatalf("failed to create service: %v", err)
	}

	proto.RegisterWorkerServiceServer(grpcServer, service)

	return grpcServer, listener
}

func newTestClientConn() *grpc.ClientConn {
	opts := make([]grpc.DialOption, 0, 1)
	opts = append(opts, grpc.WithInsecure())

	connection, err := grpc.Dial("127.0.0.1:7777", opts...)

	if err != nil {
		log.Fatalf("fail to dial: %v", err)
	}

	return connection
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
