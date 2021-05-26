package auth_test

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/MinhNghiaD/jobworker/pkg/auth"
	token "github.com/libopenstorage/openstorage-sdk-auth/pkg/auth"
	"github.com/stretchr/testify/assert"
)

func TestToken(t *testing.T) {
	certificate, err := tls.LoadX509KeyPair("../../assets/cert/jwt_cert.pem", "../../assets/cert/jwt_key.pem")
	if err != nil {
		t.Error(err)
	}

	claims := &token.Claims{
		Email: "my@email.com",
		Name:  "myname",
		Roles: []string{"hello"},
	}

	// Create
	rawtoken, err := auth.GenerateToken(claims, certificate)
	assert.NoError(t, err)
	assert.NotEmpty(t, rawtoken)

	// Verify
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

	convertedClaims, err := auth.DecodeToken(rawtoken, cert)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(convertedClaims, claims) {
		t.Error("Conversion wrong")
	}
}
