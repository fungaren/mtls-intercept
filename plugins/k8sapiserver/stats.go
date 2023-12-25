// Package k8sapiserver is to analyze the traffic pass through the apiserver
// to output the statistics.
package k8sapiserver

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/fungaren/mtls-intercept/pkg/logger"
	"github.com/fungaren/mtls-intercept/plugins"
)

var (
	commonLabels = []string{"verb", "object", "ua", "source", "username"}

	responseLength = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "payload_length",
			Help: "Payload length of the response",
		},
		commonLabels,
	)
	requestLength = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "request_length",
			Help: "Payload length of the request",
		},
		commonLabels,
	)
	requestCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "request_count",
			Help: "Counter of requests",
		},
		commonLabels,
	)
)

func extractUsernameFromJWT(jwt string) string {
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		logger.L.Warn("unable to extract username from bearer token: invalid token")
		return "unknown"
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		logger.L.Warn("unable to extract username from bearer token", zap.Error(err))
		return "unknown"
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		logger.L.Warn("unable to extract username from bearer token", zap.Error(err))
		return "unknown"
	}

	if sub, ok := claims["sub"]; ok {
		if username, ok := sub.(string); ok {
			return username
		}
	}
	return "unknown"
}

func extractUsername(req *http.Request, clientCert *x509.Certificate) string {
	if clientCert == nil {
		authHeader := req.Header.Get("Authorization")
		auth := strings.Split(authHeader, " ")
		if len(auth) > 1 {
			return extractUsernameFromJWT(auth[1])
		} else {
			logger.L.Info("unable to parse authorization header", zap.String("Authorization", authHeader))
			return "unknown"
		}
	} else {
		o := clientCert.Subject.Organization
		if len(o) > 0 {
			return clientCert.Subject.CommonName + "." + o[0]
		} else {
			return clientCert.Subject.CommonName
		}
	}
}

type k8sAPIServerStats struct {
}

func init() {
	plugins.Register(&k8sAPIServerStats{})
}

func (k *k8sAPIServerStats) Name() string {
	return "k8sapiserver"
}

func (k *k8sAPIServerStats) Setup() {
	// Register Prometheus metrics
	prometheus.MustRegister(responseLength, requestLength, requestCount)

	http.Handle("/metrics", promhttp.Handler())

	go func() {
		_ = http.ListenAndServe(":9001", nil)
	}()

	logger.L.Info("k8sapiserver metrics exported at :9001")
}

func (k *k8sAPIServerStats) OnRequest(req *http.Request, clientCert *x509.Certificate) {
	verb := req.Method
	if strings.Index(req.RequestURI, "watch=true") > 0 {
		verb = "WATCH"
	}
	object := req.URL.Path
	ua := strings.Split(req.Header.Get("User-Agent"), "/")[0]
	source := strings.Split(req.RemoteAddr, ":")[0]
	username := extractUsername(req, clientCert)

	if req.ContentLength > 0 {
		requestLength.WithLabelValues(verb, object, ua, source, username).Add(float64(req.ContentLength))
	} else {
		requestLength.WithLabelValues(verb, object, ua, source, username).Add(0)
	}
	requestCount.WithLabelValues(verb, object, ua, source, username).Inc()
}

func (k *k8sAPIServerStats) OnResponse(resp *http.Response, clientCert *x509.Certificate) {
	verb := resp.Request.Method
	if strings.Index(resp.Request.RequestURI, "watch=true") > 0 {
		verb = "WATCH"
	}
	object := resp.Request.URL.Path
	ua := strings.Split(resp.Request.Header.Get("User-Agent"), "/")[0]
	source := strings.Split(resp.Request.RemoteAddr, ":")[0]
	username := extractUsername(resp.Request, clientCert)

	if resp.ContentLength > 0 {
		responseLength.WithLabelValues(verb, object, ua, source, username).Add(float64(resp.ContentLength))
	} else {
		data, err := io.ReadAll(resp.Body)
		if err != nil && !errors.Is(err, io.ErrClosedPipe) {
			logger.L.Warn("error when reading response body", zap.Error(err))
		}
		responseLength.WithLabelValues(verb, object, ua, source, username).Add(float64(len(data)))
	}
}
