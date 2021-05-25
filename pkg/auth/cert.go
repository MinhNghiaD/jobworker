package auth

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"time"
)

// Certs contains certificates and CA certificates pool
type Certs struct {
	Certificate *tls.Certificate
	CAPool      *x509.CertPool
}

// LoadCerts loads certificates from files given by the arguments
func LoadCerts(certFile string, keyFile string, caCertFiles []string) (*Certs, error) {
	// Load server's certificate and private key
	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	certPool := x509.NewCertPool()
	for _, caFile := range caCertFiles {
		// Load certificate of the CA
		caCert, err := ioutil.ReadFile(caFile)
		if err != nil {
			return nil, err
		}

		if !certPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to add CA's certificate")
		}
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
