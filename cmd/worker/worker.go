package main

import (
	"flag"

	"github.com/MinhNghiaD/jobworker/pkg/service"
	"github.com/sirupsen/logrus"
)

var (
	port *int
)

func init() {
	// TODO add options for certificate, private key and client CAs
	port = flag.Int("port", 7777, "port number")
}

func main() {
	flag.Parse()

	server, err := service.NewServer(*port)
	if err != nil {
		logrus.Fatal(err)
	}

	defer server.Close()
	server.Serve()
}
