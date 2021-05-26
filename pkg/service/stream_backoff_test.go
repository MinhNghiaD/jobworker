package service

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/MinhNghiaD/jobworker/api/client"
	"github.com/MinhNghiaD/jobworker/api/worker/proto"
	"github.com/MinhNghiaD/jobworker/pkg/auth"
)

// This test is internal test of connection backoff implementation. This test start a job that counting number from 0.
// The client will request the log stream from this job and use the number counted in the log as the sequence number.
// We will simulate connection interruption and see if the client is capable of resuming the stream.
func TestStreamBackoff(t *testing.T) {
	server, cli := initTestServerClient(t)
	go server.Serve()

	defer t.Cleanup(func() {
		server.Close()
		cli.Close()
	})

	// start a job that count from 0 to 1000 then use it to compare the sequence received by the stream receiver
	command := &proto.Command{
		Cmd:  "bash",
		Args: []string{"-c", "for i in `seq 0 1000`; do echo $i; sleep 0.01; done"},
	}

	j, err := cli.StartJob(context.Background(), command)
	if err != nil {
		t.Error(err)
	}

	// Init the stream
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	receiver, err := cli.GetLogReceiver(ctx, j)
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

		// Verify sequence received by the client
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

// initTestServerClient creates test server and test client.
// If the call is success, it will return the server and client for testing.
// If it fail, neither of them is returned
func initTestServerClient(t *testing.T) (*WorkerServer, *client.Client) {
	serverCert, err := auth.LoadCerts(
		"../../assets/cert/server_cert.pem",
		"../../assets/cert/server_key.pem",
		[]string{"../../assets/cert/client_ca_cert.pem"},
	)
	if err != nil {
		t.Fatal(err)
	}

	cliCert, err := auth.LoadCerts(
		"../../assets/cert/user1_cert.pem",
		"../../assets/cert/user1_key.pem",
		[]string{"../../assets/cert/server_ca_cert.pem"},
	)
	if err != nil {
		t.Fatal(err)
	}

	jwtCert, err := auth.ReaderCertFile("../../assets/cert/jwt_cert.pem")
	if err != nil {
		t.Fatal(err)
	}

	// Using admin to initiate certain jobs for other user testing
	adminToken, err := ioutil.ReadFile("../../assets/jwt/admin.jwt")
	if err != nil {
		t.Fatal(err)
	}

	server, err := NewServer(7777)
	if err != nil {
		t.Fatal(err)
	}

	cli, err := client.NewWithTLS("127.0.0.1:7777", cliCert.ClientTLSConfig())
	if err != nil {
		t.Fatal(err)
	}

	server.AddAuthentication(serverCert.ServerTLSConfig())
	server.AddAuthorization(jwtCert)
	cli.UseToken(string(adminToken))

	return server, cli
}
