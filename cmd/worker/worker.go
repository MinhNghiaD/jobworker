package main

import (
	"github.com/MinhNghiaD/jobworker/pkg/service"
	"github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	port = kingpin.Flag("port", "server port").Default("7777").Int()
	// TODO add flags for certificate, private key and server CAs
)

func main() {
	kingpin.Parse()

	server, err := service.NewServer(*port)
	if err != nil {
		logrus.Fatal(err)
	}

	defer func() {
		if err := server.Close(); err != nil {
			logrus.Error(err)
		}
	}()

	if err := server.Serve(); err != nil {
		logrus.Fatal(err)
	}
}
