package service_test

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	pb "google.golang.org/protobuf/proto"

	"github.com/MinhNghiaD/jobworker/api/client"
	"github.com/MinhNghiaD/jobworker/api/worker/proto"
	"github.com/MinhNghiaD/jobworker/pkg/service"
	"github.com/sirupsen/logrus"
)

func TestStreaming(t *testing.T) {
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
	checkStream := func(t *testing.T, cmd string, args []string, forceStop bool) {
		command := &proto.Command{
			Cmd:  cmd,
			Args: args,
		}

		j, err := client.Stub.StartJob(context.Background(), command)
		if err != nil {
			t.Error(err)
		}

		logrus.Infof("Started job %s", j)

		logResults := make([]([]*proto.Log), 10)

		var wg sync.WaitGroup
		for i := 0; i < len(logResults); i++ {
			wg.Add(1)

			go func(index int) {
				defer wg.Done()
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				stream, err := client.Stub.StreamLog(ctx, &proto.StreamRequest{Job: j, StartPoint: 0})
				if err != nil {
					t.Error(err)
				}

				logResults[index] = readLog(t, stream)
			}(i)
		}

		// Let the jobs run for 5 seconds to collect enough logs
		time.Sleep(5 * time.Second)

		client.Stub.StopJob(context.Background(), &proto.StopRequest{Job: j, Force: forceStop})

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

// readLog reads logs from the job
func readLog(t *testing.T, logStream proto.WorkerService_StreamLogClient) []*proto.Log {
	readText := make([]*proto.Log, 0)
	for {
		line, err := logStream.Recv()
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
