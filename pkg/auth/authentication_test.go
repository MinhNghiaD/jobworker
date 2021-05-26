package auth_test

import (
	"context"
	"crypto/tls"
	"io/ioutil"
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
func TestVerifyCertificate(t *testing.T) {
	connectionCheck := func(serverTLSConfig *tls.Config, clientTLSConfig *tls.Config) error {
		server, cli := initTestServerClient(t, serverTLSConfig, clientTLSConfig)
		server.AddAuthentication(serverTLSConfig)
		go server.Serve()

		defer server.Close()
		defer cli.Close()

		_, err := cli.StartJob(context.Background(), &proto.Command{
			Cmd:  "ls",
			Args: nil,
		})

		return err
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
			"../../assets/cert/user1_cert.pem",
			"../../assets/cert/user1_key.pem",
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
			"../../assets/cert/user1_cert.pem",
			"../../assets/cert/user1_key.pem",
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
			"../../assets/cert/user1_cert.pem",
			"../../assets/cert/user1_key.pem",
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

			err = connectionCheck(serverCert.ServerTLSConfig(), clientCert.ClientTLSConfig())

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

// TestVerifyProtocol verifies the mTLS configuration accept only TLS 1.3 as protocol
func TestVerifyProtocol(t *testing.T) {
	serverCert, err := auth.LoadCerts(
		"../../assets/cert/server_cert.pem",
		"../../assets/cert/server_key.pem",
		[]string{"../../assets/cert/client_ca_cert.pem"},
	)
	if err != nil {
		t.Fatal(err)
	}

	server, err := service.NewServer(7777)
	if err != nil {
		t.Fatal(err)
	}

	server.AddAuthentication(serverCert.ServerTLSConfig())
	go server.Serve()

	cli1Cert, err := auth.LoadCerts(
		"../../assets/cert/user1_cert.pem",
		"../../assets/cert/user1_key.pem",
		[]string{"../../assets/cert/server_ca_cert.pem"},
	)
	if err != nil {
		t.Error(err)
	}

	cli1, err := client.NewWithTLS("127.0.0.1:7777", &tls.Config{
		Certificates: []tls.Certificate{*cli1Cert.Certificate},
		RootCAs:      cli1Cert.CAPool,
		MaxVersion:   tls.VersionTLS12,
	})
	if err != nil {
		t.Error(err)
	}

	_, err = cli1.StartJob(context.Background(), &proto.Command{
		Cmd:  "ls",
		Args: nil,
	})

	if err == nil || status.Convert(err).Code() != codes.Unavailable {
		t.Errorf("Error %s, while expected error code %v", err, codes.Unavailable)
	}

	cli2, err := client.NewWithInsecure("127.0.0.1:7777")
	if err != nil {
		t.Error(err)
	}

	_, err = cli2.StartJob(context.Background(), &proto.Command{
		Cmd:  "ls",
		Args: nil,
	})

	if err == nil || status.Convert(err).Code() != codes.Unavailable {
		t.Errorf("Error %s, while expected error code %v", err, codes.Unavailable)
	}

	t.Cleanup(func() {
		server.Close()
		cli1.Close()
		cli2.Close()
	})
}

// TestExpiration verifies the mTLS configuration accept only valid certificate
func TestExpiration(t *testing.T) {
	serverCert, err := auth.LoadCerts(
		"../../assets/cert/server_cert.pem",
		"../../assets/cert/server_key.pem",
		[]string{"../../assets/cert/client_ca_cert.pem"},
	)
	if err != nil {
		t.Fatal(err)
	}

	clientCert, err := auth.LoadCerts(
		"../../assets/cert/user1_cert.pem",
		"../../assets/cert/user1_key.pem",
		[]string{"../../assets/cert/server_ca_cert.pem"},
	)
	if err != nil {
		t.Fatal(err)
	}

	// Increment the date to simulate a certificate expiration
	nextYear := time.Now().AddDate(1, 0, 0)
	clientTLSConfig := clientCert.ClientTLSConfig()
	clientTLSConfig.Time = nextYear.Local

	server, cli := initTestServerClient(t, serverCert.ServerTLSConfig(), clientTLSConfig)

	go server.Serve()

	_, err = cli.StartJob(context.Background(), &proto.Command{
		Cmd:  "ls",
		Args: nil,
	})

	if err == nil || status.Convert(err).Code() != codes.Unavailable {
		t.Errorf("Error %s, while expected error code %v", err, codes.Unavailable)
	}

	t.Cleanup(func() {
		server.Close()
		cli.Close()
	})
}

// initTestServerClient creates test server and test client.
// If the call is success, it will return the server and client for testing.
// If it fail, neither of them is returned
func initTestServerClient(t *testing.T, serverTLS *tls.Config, clientTLS *tls.Config) (*service.WorkerServer, *client.Client) {
	server, err := service.NewServer(7777)
	if err != nil {
		t.Fatal(err)
	}

	cli, err := client.NewWithTLS("127.0.0.1:7777", clientTLS)
	if err != nil {
		t.Fatalf("Fail to init client %s", err)
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

	server.AddAuthentication(serverTLS)
	server.AddAuthorization(jwtCert)
	cli.UseToken(string(adminToken))

	return server, cli
}
