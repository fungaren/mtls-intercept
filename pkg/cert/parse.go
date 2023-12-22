package cert

import (
	"crypto/tls"
	"crypto/x509"
	"os"
)

func LoadCaPEM(caCertFile, caKeyFile string) (*tls.Certificate, error) {
	rawCert, err := os.ReadFile(caCertFile)
	if err != nil {
		return nil, err
	}
	rawKey, err := os.ReadFile(caKeyFile)
	if err != nil {
		return nil, err
	}

	ca, err := tls.X509KeyPair(rawCert, rawKey)
	if err != nil {
		return nil, err
	}
	if ca.Leaf, err = x509.ParseCertificate(ca.Certificate[0]); err != nil {
		return nil, err
	}
	return &ca, nil
}
