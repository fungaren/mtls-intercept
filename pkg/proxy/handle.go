package proxy

import (
	"bufio"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/fungaren/mtls-intercept/pkg/logger"
	"go.uber.org/zap"
)

const (
	connectUpstreamTimeout = 5 * time.Second

	errNetClosing = "use of closed network connection"
)

func (p *Proxy) proxyToUpstream(client net.Conn, clientCert *tls.Certificate) {
	tlsDialer := tls.Dialer{
		NetDialer: &net.Dialer{
			Deadline: time.Now().Add(connectUpstreamTimeout),
		},
		Config: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	if clientCert != nil {
		// If the client provided a certificate, pass it to the upstream.
		tlsDialer.Config.Certificates = []tls.Certificate{*clientCert}
	}

	remote, err := tlsDialer.Dial("tcp", p.upstream)
	if err != nil {
		if err != io.EOF {
			logger.L.Warn("error connecting to remote", zap.Error(err))
		}
		client.Close()
		return
	}

	if clientCert != nil {
		logger.L.Debug("+crt", zap.String("client", client.RemoteAddr().String()),
			zap.String("common name", clientCert.Leaf.Subject.CommonName))
	} else {
		logger.L.Debug("-crt", zap.String("client", client.RemoteAddr().String()))
	}

	pipeReqReader, pipeReqWriter := io.Pipe()
	reqReader := io.TeeReader(client, pipeReqWriter)

	pipeRespReader, pipeRespWriter := io.Pipe()
	respReader := io.TeeReader(remote, pipeRespWriter)

	go func() {
		_, err := io.Copy(client, respReader)
		if err != nil {
			if !strings.Contains(err.Error(), errNetClosing) {
				logger.L.Warn("error when reading from remote", zap.Error(err))
			}
		}
		_ = client.Close()
		_ = pipeRespReader.Close()
	}()

	go func() {
		_, err := io.Copy(remote, reqReader)
		if err != nil {
			if !strings.Contains(err.Error(), errNetClosing) {
				logger.L.Warn("error when reading from client", zap.Error(err))
			}
		}
		_ = remote.Close()
		_ = pipeReqReader.Close()
	}()

	go func() {
		for {
			req, err := http.ReadRequest(bufio.NewReader(pipeReqReader))
			if err != nil {
				if !errors.Is(err, io.ErrClosedPipe) {
					logger.L.Error("error when read request", zap.Error(err))
				}
				return
			}
			// This field is not filled in by ReadRequest.
			req.RemoteAddr = client.RemoteAddr().String()

			logger.L.Debug("<---", zap.String("client", req.RemoteAddr),
				zap.String("method", req.Method), zap.String("url", req.RequestURI))

			if clientCert != nil {
				p.onRequest(req, clientCert.Leaf)
			} else {
				p.onRequest(req, nil)
			}

			resp, err := http.ReadResponse(bufio.NewReader(pipeRespReader), req)
			if err != nil {
				logger.L.Error("error when read response", zap.Error(err))
				return
			}

			logger.L.Debug("--->", zap.String("client", req.RemoteAddr),
				zap.String("status", resp.Status))

			if clientCert != nil {
				p.onResponse(resp, clientCert.Leaf)
			} else {
				p.onResponse(resp, nil)
			}
		}
	}()
}
