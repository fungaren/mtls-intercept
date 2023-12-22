package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"

	"go.uber.org/zap"

	"github.com/fungaren/mtls-intercept/pkg/cert"
	"github.com/fungaren/mtls-intercept/pkg/logger"
	"github.com/fungaren/mtls-intercept/plugins"
)

type Proxy struct {
	listenPort int
	upstream   string
	serverCert *tls.Certificate
	clientCA   *tls.Certificate

	isRunning   bool
	tlsListener net.Listener

	// The callback for client requests to eavesdrop.
	onRequest func(req *http.Request, clientCert *x509.Certificate)
	// The callback for server response to eavesdrop. The callback is responsible to close the resp.Body.
	onResponse func(resp *http.Response, clientCert *x509.Certificate)
}

func NewProxy(verbose bool, listenPort int, upstream string, serverCert, clientCA *tls.Certificate) *Proxy {
	p := &Proxy{
		listenPort: listenPort,
		upstream:   upstream,
		serverCert: serverCert,
		clientCA:   clientCA,

		onRequest:  plugins.OnRequest,
		onResponse: plugins.OnResponse,
	}
	return p
}

func (p *Proxy) Start() error {
	var err error

	// listen to the TLS ClientHello but make it a CONNECT request instead
	certPool := x509.NewCertPool()
	certPool.AddCert(p.clientCA.Leaf)

	p.tlsListener, err = tls.Listen("tcp", fmt.Sprint(":", p.listenPort), &tls.Config{
		Certificates: []tls.Certificate{*p.serverCert},
		ClientCAs:    certPool,
		MinVersion:   tls.VersionTLS12,
		// Request the client to provide certificate, but we will not verify it.
		// The client can do not send any certificate, to authenticate itself by
		// HTTP Authorization header.
		ClientAuth: tls.RequestClientCert,
	})
	if err != nil {
		return fmt.Errorf("error listening for https connections: %s", err.Error())
	}
	logger.L.Info("server started", zap.Int("listen port", p.listenPort))

	p.isRunning = true
	for p.isRunning {
		c, err := p.tlsListener.Accept()
		if err != nil {
			logger.L.Warn("error accepting connection", zap.Error(err))
			continue
		}

		go func(c net.Conn) {
			tlsConn, _ := c.(*tls.Conn)
			if err := tlsConn.Handshake(); err != nil {
				logger.L.Warn("tls handshake with the client failed",
					zap.String("from", c.RemoteAddr().String()), zap.Error(err))
				_ = c.Close()
				return
			}

			clientCerts := tlsConn.ConnectionState().PeerCertificates
			var clientCert *tls.Certificate
			if len(clientCerts) == 0 {
				// client didn't provide the certificate
			} else {
				clientCert, err = cert.SignCertificate(p.clientCA, clientCerts[0])
				if err != nil {
					logger.L.Warn("failed to sign certificate for the client",
						zap.String("from", c.RemoteAddr().String()), zap.Error(err))
					_ = c.Close()
					return
				}
			}

			p.proxyToUpstream(c, clientCert)
		}(c)
	}

	return nil
}

func (p *Proxy) Stop() error {
	p.isRunning = false
	return p.tlsListener.Close()
}
