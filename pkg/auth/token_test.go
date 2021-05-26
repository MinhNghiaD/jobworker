package auth_test

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
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

	pemServer, err := ioutil.ReadFile("../../assets/cert/jwt_cert.pem")
	if err != nil {
		t.Fatal(err)
	}

	block, _ := pem.Decode(pemServer)
	if block == nil {
		t.Fatal("failed to parse certificate PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}

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
