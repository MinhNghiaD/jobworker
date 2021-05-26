package auth_test

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/libopenstorage/openstorage-sdk-auth/pkg/auth"
	"github.com/stretchr/testify/assert"
)

func TestToken(t *testing.T) {
	certificate, err := tls.LoadX509KeyPair("../../assets/cert/server_cert.pem", "../../assets/cert/server_key.pem")
	if err != nil {
		t.Error(err)
	}

	claims := auth.Claims{
		Email: "my@email.com",
		Name:  "myname",
		Roles: []string{"hello"},
	}

	sig := auth.Signature{
		Type: jwt.SigningMethodES256,
		Key:  certificate.PrivateKey,
	}
	opts := auth.Options{
		Expiration: time.Now().Add(time.Minute * 10).Unix(),
	}

	// Create
	rawtoken, err := auth.Token(&claims, &sig, &opts)
	assert.NoError(t, err)
	assert.NotEmpty(t, rawtoken)

	// Verify
	pemServer, err := ioutil.ReadFile("../../assets/cert/server_cert.pem")
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

	token, err := jwt.Parse(rawtoken, func(jwtToken *jwt.Token) (interface{}, error) {
		if _, ok := jwtToken.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, fmt.Errorf("unexpected method: %s", jwtToken.Header["alg"])
		}

		return cert.PublicKey, nil
	})

	assert.True(t, token.Valid)
	tokenClaims, ok := token.Claims.(jwt.MapClaims)
	assert.True(t, ok)
	assert.Contains(t, tokenClaims, "email")
	assert.Equal(t, claims.Email, tokenClaims["email"])
	assert.Contains(t, tokenClaims, "name")
	assert.Equal(t, claims.Name, tokenClaims["name"])
	assert.Contains(t, tokenClaims, "roles")
	assert.Equal(t, claims.Roles[0], tokenClaims["roles"].([]interface{})[0].(string))
}
