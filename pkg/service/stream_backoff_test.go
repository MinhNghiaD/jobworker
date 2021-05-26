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
)

// This test is internal test of connection backoff implementation. This test start a job that counting number from 0.
// The client will request the log stream from this job and use the number counted in the log as the sequence number.
// We will simulate connection interruption and see if the client is capable of resuming the stream.
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

	defer t.Cleanup(func() {
		server.Close()
		c.Close()
	})

	// start a job that count from 0 to 1000 then use it to compare the sequence received by the stream receiver
	command := &proto.Command{
		Cmd:  "bash",
		Args: []string{"-c", "for i in `seq 0 1000`; do echo $i; sleep 0.01; done"},
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
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}

			server.grpcServer.Stop()

			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}

			newListener, err := net.Listen("tcp", server.listener.Addr().String())
			if err != nil {
				t.Error(err)
			}

			server.listener = newListener
			go server.Serve()
		}
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
			if err == nil {
				// Compare the received sequence
				if sequence != counter {
					t.Errorf("Sequence mismatch %d != %d", sequence, counter)
					break
				}

				counter++
			}
		}
	}
}
