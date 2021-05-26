package main

import (
	"crypto/tls"
	"os"

	"github.com/MinhNghiaD/jobworker/pkg/auth"
	token "github.com/libopenstorage/openstorage-sdk-auth/pkg/auth"
	"github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/yaml.v2"
)

var (
	issuer = kingpin.Flag("issuer", "Issuer name of token."+
		" Do not use https:// in the issuer since it could indicate that this is an OpenID Connect issuer.").Default("jobworker").String()
	cert = kingpin.Flag("cert", "certificate file").Default("./assets/cert/jwt_cert.pem").String()
	key  = kingpin.Flag("key", "private key file").Default("./assets/cert/jwt_key.pem").String()
	in   = kingpin.Flag("in", "input claim yaml file").Default("./assets/claims/user.yaml").String()
	out  = kingpin.Flag("out", "output file").Default("./assets/jwt/user.jwt").String()
)

func main() {
	kingpin.Parse()
	claimFile, err := os.Open(*in)
	if err != nil {
		logrus.Fatal("Fail to open claim file, %s", err)
	}

	defer claimFile.Close()

	// Start YAML decoding from file
	decoder := yaml.NewDecoder(claimFile)
	var claims token.Claims
	if err := decoder.Decode(&claims); err != nil {
		logrus.Fatal("Fail to parse claim file, %s", err)
	}

	// Mark claim with the issuer
	claims.Issuer = *issuer

	certificate, err := tls.LoadX509KeyPair(*cert, *key)
	if err != nil {
		logrus.Fatal("Fail to load private key, %s", err)
	}

	// Signing token with private key
	rawToken, err := auth.GenerateToken(&claims, certificate)
	if err != nil {
		logrus.Fatal("Fail to generate token, %s", err)
	}

	outputFile, err := os.OpenFile(*out, (os.O_CREATE | os.O_WRONLY), 0644)
	if err != nil {
		logrus.Fatal("Fail to create output file, %s", err)
	}

	_, err = outputFile.WriteString(rawToken)
	if err != nil {
		logrus.Fatal("Fail to write to output file, %s", err)
	}
}
