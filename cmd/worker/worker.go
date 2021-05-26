package main

import (
	"github.com/MinhNghiaD/jobworker/pkg/auth"
	"github.com/MinhNghiaD/jobworker/pkg/service"
	"github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	port          = kingpin.Flag("port", "server port").Default("7777").Int()
	jwtCertFile   = kingpin.Flag("jwtkey", "JWT signing certificate file").Default("./assets/cert/jwt_cert.pem").ExistingFile()
	certFile      = kingpin.Flag("cert", "certificate file").Default("./assets/cert/server_cert.pem").ExistingFile()
	keyFile       = kingpin.Flag("key", "private key file").Default("./assets/cert/server_key.pem").ExistingFile()
	clientCA      = kingpin.Flag("ca", "client trusted certifiacte authorities files").Required().ExistingFile()
	additionalCAs = kingpin.Arg("ca", "client trusted certifiacte authorities files").Required().ExistingFiles()
)

func main() {
	kingpin.Parse()

	*additionalCAs = append(*additionalCAs, *clientCA)
	serverCert, err := auth.LoadCerts(*certFile, *keyFile, *additionalCAs)
	if err != nil {
		logrus.Fatal(err)
	}

	jwtCert, err := auth.ReaderCertFile(*jwtCertFile)
	if err != nil {
		logrus.Fatal(err)
	}

	server, err := service.NewServer(*port)
	if err != nil {
		logrus.Fatal(err)
	}

	server.AddAuthentication(serverCert.ServerTLSConfig())
	server.AddAuthorization(jwtCert)

	defer func() {
		if err := server.Close(); err != nil {
			logrus.Error(err)
		}
	}()

	if err := server.Serve(); err != nil {
		logrus.Fatal(err)
	}

}
