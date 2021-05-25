package auth_test

import (
	"context"
	"crypto/tls"
	"testing"

	"github.com/MinhNghiaD/jobworker/api/client"
	"github.com/MinhNghiaD/jobworker/api/worker/proto"
	"github.com/MinhNghiaD/jobworker/pkg/auth"
	"github.com/MinhNghiaD/jobworker/pkg/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestVerifyCertificate(t *testing.T) {
	connectionCheck := func(serverTLSConfig *tls.Config, clientTLSConfig *tls.Config) error {
		server, err := service.NewServer(7777, serverTLSConfig)
		if err != nil {
			return err
		}

		go server.Serve()
		defer server.Close()

		cli, err := client.NewWithTLS("127.0.0.1:7777", clientTLSConfig)
		if err != nil {
			return err
		}

		command := &proto.Command{
			Cmd:  "ls",
			Args: nil,
		}

		_, err = cli.StartJob(context.Background(), command)
		if err != nil {
			return err
		}

		cli.Close()
		return nil
	}

	testcases := []struct {
		name           string
		serverCertFile string
		serverKeyFile  string
		serverCAFiles  []string
		clientCertFile string
		clientKeyFile  string
		clientCAFiles  []string
		expectErrCode  codes.Code
	}{
		{
			"happy",
			"../../assets/cert/server_cert.pem",
			"../../assets/cert/server_key.pem",
			[]string{"../../assets/cert/server_ca_cert.pem"},
			"../../assets/cert/admin_cert.pem",
			"../../assets/cert/admin_key.pem",
			[]string{"../../assets/cert/client_ca_cert.pem"},
			codes.OK,
		},
		{
			"self-signed client",
			"../../assets/cert/server_cert.pem",
			"../../assets/cert/server_key.pem",
			[]string{"../../assets/cert/server_ca_cert.pem"},
			"../../assets/cert/selfsigned_client_cert.pem",
			"../../assets/cert/selfsigned_client_key.pem",
			[]string{"../../assets/cert/client_ca_cert.pem"},
			codes.Unavailable,
		},
		{
			"self-signed server",
			"../../assets/cert/selfsigned_server_cert.pem",
			"../../assets/cert/selfsigned_server_key.pem",
			[]string{"../../assets/cert/server_ca_cert.pem"},
			"../../assets/cert/observer_cert.pem",
			"../../assets/cert/observer_key.pem",
			[]string{"../../assets/cert/client_ca_cert.pem"},
			codes.Unavailable,
		},
		{
			"untrusted client",
			"../../assets/cert/server_cert.pem",
			"../../assets/cert/server_key.pem",
			[]string{"../../assets/cert/server_ca_cert.pem"},
			"../../assets/cert/untrusted_client_cert.pem",
			"../../assets/cert/untrusted_client_key.pem",
			[]string{"../../assets/cert/client_ca_cert.pem"},
			codes.Unavailable,
		},
		{
			"untrusted server",
			"../../assets/cert/untrusted_server_cert.pem",
			"../../assets/cert/untrusted_server_key.pem",
			[]string{"../../assets/cert/server_ca_cert.pem"},
			"../../assets/cert/observer_cert.pem",
			"../../assets/cert/observer_key.pem",
			[]string{"../../assets/cert/client_ca_cert.pem"},
			codes.Unavailable,
		},
	}

	for _, testCase := range testcases {
		t.Run(testCase.name, func(t *testing.T) {
			serverCert, err := auth.LoadCerts(testCase.serverCertFile, testCase.serverKeyFile, testCase.clientCAFiles)
			if err != nil {
				t.Error(err)
				return
			}

			clientCert, err := auth.LoadCerts(testCase.clientCertFile, testCase.clientKeyFile, testCase.serverCAFiles)
			if err != nil {
				t.Error(err)
				return
			}

			err = connectionCheck(serverCert.ServerTLSConfig(), clientCert.ClientTLSConfig().Clone())

			if err != nil {
				s := status.Convert(err)
				if s.Code() != testCase.expectErrCode {
					t.Errorf("Test case %s returns error %v, while %v code is expected", testCase.name, err, testCase.expectErrCode)
				}
			} else if testCase.expectErrCode != codes.OK {
				t.Errorf("Test case %s returns no error, while %v error is expected", testCase.name, testCase.expectErrCode)
			}
		})
	}
}
