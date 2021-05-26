package auth_test

import (
	"context"
	"io"
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

// TestRBAC verifies RBAC rules predefined by the default configuration.
// It ensures that admin has all access to all APIs on all jobs.
// User has access right to Start/Query APIs and can only Stop/Stream the their jobs.
// Observer can only Query jobs
// Other undefined roles cannot access the system
// Invalid token signed by unknown party cannot access the system
func TestRBAC(t *testing.T) {
	jwtCert, err := auth.ReaderCertFile("../../assets/cert/jwt_cert.pem")
	if err != nil {
		t.Fatal(err)
	}

	server, err := service.NewServer(7777)
	if err != nil {
		t.Fatal(err)
	}

	server.AddAuthorization(jwtCert)
	go server.Serve()

	// Using admin to initiate certain jobs for other user testing
	adminToken, err := ioutil.ReadFile("../../assets/jwt/admin.jwt")
	if err != nil {
		t.Error(err)
	}
	admin, err := client.NewWithInsecure("127.0.0.1:7777")
	admin.UseToken(string(adminToken))

	if err != nil {
		t.Error(err)
		return
	}

	defer t.Cleanup(func() {
		server.Close()
		admin.Close()
	})

	type action *func(*client.Client) error
	rbacCheck := func(t *testing.T, rawToken string, expectedErrors map[action]codes.Code) {
		cli, err := client.NewWithInsecure("127.0.0.1:7777")
		if err != nil {
			t.Error(err)
			return
		}

		cli.UseToken(rawToken)
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

	// helper functions perform the 4 basic RPCs
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
		tokenFile      string
		expectedErrors map[action]codes.Code
	}{
		{
			"admin",
			"../../assets/jwt/admin.jwt",
			map[action]codes.Code{
				&start:  codes.OK,
				&stop:   codes.OK,
				&query:  codes.OK,
				&stream: codes.OK,
			},
		},
		{
			"observer",
			"../../assets/jwt/observer.jwt",
			map[action]codes.Code{
				&start:  codes.PermissionDenied,
				&stop:   codes.PermissionDenied,
				&query:  codes.OK,
				&stream: codes.PermissionDenied,
			},
		},
		{
			// User can ont stop and stream their own jobs
			"user",
			"../../assets/jwt/user.jwt",
			map[action]codes.Code{
				&start:  codes.OK,
				&stop:   codes.PermissionDenied,
				&query:  codes.OK,
				&stream: codes.PermissionDenied,
			},
		},
		{
			"unknown role",
			"../../assets/jwt/unknown_role.jwt",
			map[action]codes.Code{
				&start:  codes.PermissionDenied,
				&stop:   codes.PermissionDenied,
				&query:  codes.PermissionDenied,
				&stream: codes.PermissionDenied,
			},
		},
		{
			"invalid token",
			"../../assets/jwt/invalid.jwt",
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
			rawToken, err := ioutil.ReadFile(testCase.tokenFile)
			if err != nil {
				t.Error(err)
			}

			rbacCheck(t, string(rawToken), testCase.expectedErrors)
		})
	}
}
