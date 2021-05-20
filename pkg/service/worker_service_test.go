package service_test

import (
	"context"
	"log"
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

	// checker
	checkStartJob := func(cmd string, args []string) error {
		clientConn := newTestClientConn()
		defer clientConn.Close()
		client := proto.NewWorkerServiceClient(clientConn)

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
