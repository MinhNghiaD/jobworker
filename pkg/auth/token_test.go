package auth_test

import (
	"crypto/tls"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/MinhNghiaD/jobworker/pkg/auth"
	token "github.com/libopenstorage/openstorage-sdk-auth/pkg/auth"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

func TestToken(t *testing.T) {
	keyPair, err := tls.LoadX509KeyPair("../../assets/cert/jwt_cert.pem", "../../assets/cert/jwt_key.pem")
	if err != nil {
		t.Error(err)
	}

	cert, err := auth.ReaderCertFile("../../assets/cert/jwt_cert.pem")

	checker := func(fileName string) error {
		claimFile, err := os.Open(fileName)
		if err != nil {
			return err
		}

		defer claimFile.Close()

		// Start YAML decoding from file
		decoder := yaml.NewDecoder(claimFile)
		var claims token.Claims
		if err := decoder.Decode(&claims); err != nil {
			return err
		}

		rawtoken, err := auth.GenerateToken(&claims, keyPair)
		if err != nil {
			return err
		}

		// Verify
		convertedClaims, err := auth.DecodeToken(rawtoken, cert)
		if err != nil {
			return err
		}

		if !reflect.DeepEqual(*convertedClaims, claims) {
			logrus.Info(convertedClaims)
			logrus.Info("!=")
			logrus.Info(claims)
			return fmt.Errorf("Conversion wrong")
		}

		return nil
	}

	testcases := []struct {
		name      string
		claimFile string
	}{
		{
			"user",
			"../../assets/claims/user.yaml",
		},
		{
			"admin",
			"../../assets/claims/admin.yaml",
		},
		{
			"observer",
			"../../assets/claims/observer.yaml",
		},
		{
			"user2",
			"../../assets/claims/user2.yaml",
		},
		{
			"unknown",
			"../../assets/claims/unknown_role.yaml",
		},
	}

	for _, testCase := range testcases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if err := checker(testCase.claimFile); err != nil {
				t.Error(err)
			}
		})
	}
}
