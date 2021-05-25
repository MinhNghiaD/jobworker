package auth_test

import (
	"context"
	"crypto/tls"
	"io"
	"testing"
	"time"

	"github.com/MinhNghiaD/jobworker/api/client"
	"github.com/MinhNghiaD/jobworker/api/worker/proto"
	"github.com/MinhNghiaD/jobworker/pkg/auth"
	"github.com/MinhNghiaD/jobworker/pkg/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestVerifyCertificate verifies mTLS authentication of the service between trusted parties
func TestRBAC(t *testing.T) {
	serverCert, err := auth.LoadCerts(
		"../../assets/cert/server_cert.pem",
		"../../assets/cert/server_key.pem",
		[]string{"../../assets/cert/client_ca_cert.pem"},
	)
	if err != nil {
		t.Fatal(err)
	}

	server, err := service.NewServer(7777, serverCert.ServerTLSConfig())
	if err != nil {
		t.Fatal(err)
	}

	go server.Serve()

	adminCert, err := auth.LoadCerts(
		"../../assets/cert/admin_cert.pem",
		"../../assets/cert/admin_key.pem",
		[]string{"../../assets/cert/server_ca_cert.pem"},
	)
	if err != nil {
		t.Fatal(err)
	}

	admin, err := client.NewWithTLS("127.0.0.1:7777", adminCert.ClientTLSConfig())
	if err != nil {
		t.Error(err)
		return
	}

	defer t.Cleanup(func() {
		server.Close()
		admin.Close()
	})

	type action *func(*client.Client) error
	rbacCheck := func(t *testing.T, clientTLSConfig *tls.Config, expectedErrors map[action]codes.Code) {
		cli, err := client.NewWithTLS("127.0.0.1:7777", clientTLSConfig)
		if err != nil {
			t.Error(err)
			return
		}

		defer cli.Close()

		for act, expectedCode := range expectedErrors {
			err := (*act)(cli)
			if err != nil {
				s := status.Convert(err)
				if s.Code() != expectedCode {
					t.Errorf("Returns error %v, while %v code is expected", err, expectedCode)
				}
			} else if expectedCode != codes.OK {
				t.Errorf("Returns no error, while %v error is expected", expectedCode)
			}
		}
	}

	start := func(cli *client.Client) error {
		_, err := cli.StartJob(context.Background(), &proto.Command{Cmd: "ls"})

		return err
	}

	stop := func(cli *client.Client) error {
		// Using admin to start job for client to stop
		j, err := admin.StartJob(context.Background(), &proto.Command{Cmd: "top", Args: []string{"-b"}})
		if err != nil {
			return err
		}

		_, err = cli.StopJob(context.Background(), &proto.StopRequest{Job: j, Force: false})
		return err
	}

	query := func(cli *client.Client) error {
		// Using admin to start job for client to query
		j, err := admin.StartJob(context.Background(), &proto.Command{Cmd: "ls"})
		if err != nil {
			return err
		}

		_, err = cli.QueryJob(context.Background(), j)
		return err
	}

	stream := func(cli *client.Client) error {
		// Using admin to start job for client to stream
		j, err := admin.StartJob(context.Background(), &proto.Command{Cmd: "ps", Args: []string{"-a"}})
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		receiver, err := cli.GetLogReceiver(ctx, j)
		for err == nil {
			_, err = receiver.Read()
		}

		if err == io.EOF {
			return nil
		}

		return err
	}

	testcases := []struct {
		name           string
		serverCAFiles  []string
		clientCertFile string
		clientKeyFile  string
		expectedErrors map[action]codes.Code
	}{
		{
			"admin",
			[]string{"../../assets/cert/server_ca_cert.pem"},
			"../../assets/cert/admin_cert.pem",
			"../../assets/cert/admin_key.pem",
			map[action]codes.Code{
				&start:  codes.OK,
				&stop:   codes.OK,
				&query:  codes.OK,
				&stream: codes.OK,
			},
		},
		{
			"observer",
			[]string{"../../assets/cert/server_ca_cert.pem"},
			"../../assets/cert/observer_cert.pem",
			"../../assets/cert/observer_key.pem",
			map[action]codes.Code{
				&start:  codes.PermissionDenied,
				&stop:   codes.PermissionDenied,
				&query:  codes.OK,
				&stream: codes.OK,
			},
		},
		{
			"unauthorized",
			[]string{"../../assets/cert/server_ca_cert.pem"},
			"../../assets/cert/impostor_cert.pem",
			"../../assets/cert/impostor_key.pem",
			map[action]codes.Code{
				&start:  codes.PermissionDenied,
				&stop:   codes.PermissionDenied,
				&query:  codes.PermissionDenied,
				&stream: codes.PermissionDenied,
			},
		},
	}

	for _, testCase := range testcases {
		t.Run(testCase.name, func(t *testing.T) {
			clientCert, err := auth.LoadCerts(testCase.clientCertFile, testCase.clientKeyFile, testCase.serverCAFiles)
			if err != nil {
				t.Error(err)
				return
			}

			rbacCheck(t, clientCert.ClientTLSConfig(), testCase.expectedErrors)
		})
	}
}
