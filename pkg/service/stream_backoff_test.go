package service

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/MinhNghiaD/jobworker/api/client"
	"github.com/MinhNghiaD/jobworker/api/worker/proto"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// This tests is internal test of connection backoff implementation
func TestStreamBackoff(t *testing.T) {
	server, err := NewServer(7777)
	if err != nil {
		t.Fatal(err)
	}

	go server.Serve()

	c, err := client.New("127.0.0.1:7777")
	if err != nil {
		t.Fatalf("Fail to init client %s", err)
	}

	// start a job that count from 0 then use it to compare the sequence received by the stream receiver
	command := &proto.Command{
		Cmd:  "bash",
		Args: []string{"-c", "i=0; while true; do echo $i; let i=$i+1; sleep 1; done"},
	}

	j, err := c.StartJob(context.Background(), command)
	if err != nil {
		t.Error(err)
	}

	// Init the stream
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	receiver, err := c.GetLogReceiver(ctx, j)
	if err != nil {
		t.Error(err)
	}

	// Simulate unstable connection by interrupting the server but keep the service
	go func() {
		for i := 0; i < 3; i++ {
			time.Sleep(5 * time.Second)
			server.grpcServer.Stop()

			time.Sleep(2 * time.Second)
			server, err = resetServer(server)
			if err != nil {
				t.Error(err)
			}

			go func() {
				if err := server.Serve(); err != nil {
					t.Error(err)
				}
			}()
		}

		// Stop the job to signal the exit
		c.StopJob(context.Background(), &proto.StopRequest{Job: j, Force: true})
	}()

	counter := 0
	for {
		line, err := receiver.Read()
		if err != nil {
			if err != io.EOF {
				t.Error(err)
			}
			break
		}

		var data map[string]string
		if err := json.Unmarshal([]byte(line.Entry), &data); err != nil {
			t.Errorf("Fail to decode json format, data %s", line.Entry)
			break
		} else {
			sequence, err := strconv.Atoi(data["msg"])
			if err != nil {
				t.Error(err)
				break
			}

			// Compare the sequence received
			if sequence != counter {
				t.Errorf("Sequence mismatch %d != %d", sequence, counter)
				break
			}

			counter++
		}
	}

	t.Cleanup(func() {
		server.Close()
		c.Close()
	})
}

// resetServer switches the grpc service to a new server
func resetServer(server *WorkerServer) (*WorkerServer, error) {
	logrus.Infof("Reset listener")
	newListener, err := net.Listen("tcp", server.listener.Addr().String())
	if err != nil {
		return nil, err
	}

	opts := serverConfig()
	grpcServer := grpc.NewServer(opts...)
	proto.RegisterWorkerServiceServer(grpcServer, server.service)

	return &WorkerServer{
		grpcServer: grpcServer,
		listener:   newListener,
		service:    server.service,
	}, nil
}
