package cert

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"

	"go.uber.org/zap"

	"github.com/fungaren/mtls-intercept/pkg/logger"
)

const (
	KeyBits = 2048
)

func GetServerCertificate(endpoint string) *x509.Certificate {
	logger.L.Debug("fetching TLS certificate", zap.String("endpoint", endpoint))

	config := tls.Config{InsecureSkipVerify: true}
	conn, err := tls.Dial("tcp", endpoint, &config)
	if err != nil {
		logger.L.Error("could not fetch TLS certificate", zap.String("endpoint", endpoint), zap.Error(err))
		return nil
	}
	defer conn.Close()

	state := conn.ConnectionState()
	return state.PeerCertificates[0]
}

func SignCertificate(ca *tls.Certificate, templateCert *x509.Certificate) (cert *tls.Certificate, err error) {
	var x509ca *x509.Certificate
	var template x509.Certificate

	if x509ca, err = x509.ParseCertificate(ca.Certificate[0]); err != nil {
		return
	}

	template = x509.Certificate{
		SerialNumber:          templateCert.SerialNumber,
		Issuer:                x509ca.Subject,
		Subject:               templateCert.Subject,
		NotBefore:             templateCert.NotBefore,
		NotAfter:              templateCert.NotAfter,
		KeyUsage:              templateCert.KeyUsage,
		ExtKeyUsage:           templateCert.ExtKeyUsage,
		IPAddresses:           templateCert.IPAddresses,
		DNSNames:              templateCert.DNSNames,
		BasicConstraintsValid: true,
	}

	var certPriv *rsa.PrivateKey
	if certPriv, err = rsa.GenerateKey(rand.Reader, KeyBits); err != nil {
		return
	}

	var derBytes []byte
	if derBytes, err = x509.CreateCertificate(rand.Reader, &template, x509ca, &certPriv.PublicKey, ca.PrivateKey); err != nil {
		return
	}

	return &tls.Certificate{
		Certificate: [][]byte{derBytes, ca.Certificate[0]},
		PrivateKey:  certPriv,
		Leaf:        &template,
	}, nil
}
