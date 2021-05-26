package auth

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"time"
)

// Certs contains certificates and CA certificates pool to establish TLS configuration
type Certs struct {
	Certificate *tls.Certificate
	CAPool      *x509.CertPool
}

// LoadCerts loads certificates and key from files given by the arguments
func LoadCerts(certFile string, keyFile string, caCertFiles []string) (*Certs, error) {
	certPool := x509.NewCertPool()
	for _, caFile := range caCertFiles {
		caCert, err := ioutil.ReadFile(caFile)
		if err != nil {
			return nil, err
		}

		if !certPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to add CA's certificate")
		}
	}

	// Load server's certificate and private key
	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	return &Certs{
		Certificate: &certificate,
		CAPool:      certPool,
	}, nil
}

// ServerTLSConfig configures the default TLS policy of the server
func (cert *Certs) ServerTLSConfig() *tls.Config {
	return &tls.Config{
		Certificates:     []tls.Certificate{*cert.Certificate},
		ClientAuth:       tls.RequireAndVerifyClientCert,
		ClientCAs:        cert.CAPool,
		MinVersion:       tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
		CipherSuites: []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
		},
	}
}

// ClientTLSConfig configures the default TLS policy of the client
func (cert *Certs) ClientTLSConfig() *tls.Config {
	return &tls.Config{
		Certificates:     []tls.Certificate{*cert.Certificate},
		RootCAs:          cert.CAPool,
		MinVersion:       tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256},
		Time:             time.Now,
		CipherSuites: []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
		},
	}
}

// ReaderCertFile reads the certificate from its file and encoded in x509.Certificate object
func ReaderCertFile(certFile string) (*x509.Certificate, error) {
	certPem, err := ioutil.ReadFile(certFile)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(certPem)
	if block == nil {
		return nil, fmt.Errorf("failed to parse certificate PEM")
	}

	return x509.ParseCertificate(block.Bytes)
}
