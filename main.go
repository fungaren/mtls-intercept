package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/fungaren/mtls-intercept/pkg/cert"
	"github.com/fungaren/mtls-intercept/pkg/logger"
	"github.com/fungaren/mtls-intercept/pkg/proxy"
	"github.com/fungaren/mtls-intercept/plugins"

	// Add plugins here
	_ "github.com/fungaren/mtls-intercept/plugins/k8sapiserver"
)

var (
	commitRef string = ""
	builtTime string = ""

	port     int
	upstream string = ""
	verbose  bool

	serverCACertPEM string = ""
	serverCAKeyPEM  string = ""
	clientCACertPEM string = ""
	clientCAKeyPEM  string = ""

	rootCmd = &cobra.Command{
		Use:     "mtls-intercept",
		Short:   "reverse proxy to decrypt mTLS protected traffics.",
		Version: commitRef + " " + builtTime,
		RunE:    execute,
	}
)

func init() {
	rootCmd.Flags().IntVarP(&port, "port", "p", 8080, "listen port")
	rootCmd.Flags().StringVarP(&upstream, "upstream", "u", "", "upstream server:port")
	rootCmd.Flags().StringVar(&serverCACertPEM, "server-ca-cert", "./certs/server-ca.crt", "server ca certificate")
	rootCmd.Flags().StringVar(&serverCAKeyPEM, "server-ca-key", "./certs/server-ca.key", "server ca key")
	rootCmd.Flags().StringVar(&clientCACertPEM, "client-ca-cert", "./certs/client-ca.crt", "client ca certificate")
	rootCmd.Flags().StringVar(&clientCAKeyPEM, "client-ca-key", "./certs/client-ca.key", "client ca key")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	plugins.BindCmdFlags(rootCmd)
}

func main() {
	logger.NewZapLogger(verbose, true)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func execute(cmd *cobra.Command, args []string) error {
	plugins.Setup()

	serverCA, err := cert.LoadCaPEM(serverCACertPEM, serverCAKeyPEM)
	if err != nil {
		return fmt.Errorf("failed to load server CA PEM: %s", err.Error())
	}
	logger.L.Debug("server CA loaded", zap.String("common name", serverCA.Leaf.Subject.CommonName))

	srvCert := cert.GetServerCertificate(upstream)
	if srvCert == nil {
		return errors.New("could not fetch TLS certificate")
	}
	logger.L.Debug("got upstream server certificate", zap.String("common name", srvCert.Subject.CommonName))

	serverCert, err := cert.SignCertificate(serverCA, srvCert)
	if err != nil {
		return fmt.Errorf("failed to create the proxy cert: %s", err.Error())
	}
	logger.L.Debug("spoofed certificate created")

	clientCA, err := cert.LoadCaPEM(clientCACertPEM, clientCAKeyPEM)
	if err != nil {
		return fmt.Errorf("failed to load client CA PEM: %s", err.Error())
	}
	logger.L.Debug("client CA loaded", zap.String("common name", clientCA.Leaf.Subject.CommonName))

	proxy := proxy.NewProxy(verbose, port, upstream, serverCert, clientCA)
	go func() {
		if err := proxy.Start(); err != nil {
			logger.L.Fatal("failed to start proxy", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	logger.L.Info("received signal", zap.Any("signal", <-quit))
	if err := proxy.Stop(); err != nil {
		logger.L.Fatal("error when stopping the proxy", zap.Error(err))
	}
	return nil
}
