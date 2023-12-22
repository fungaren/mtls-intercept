package plugins

import (
	"crypto/x509"
	"net/http"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/fungaren/mtls-intercept/pkg/logger"
)

type Plugin interface {
	// Name is the plugin name displayed in the command line help text.
	Name() string
	// Setup is invoked when the program started. Plugins can initialize their
	// internal state in the hook.
	Setup()
	// OnRequest is invoked when the client sends a HTTP request to the proxy.
	// Do not modify the request object, it is readonly.
	OnRequest(req *http.Request, clientCert *x509.Certificate)
	// OnResponse is invoked when the server sends a HTTP response to the proxy.
	// Do not modify the response object, it is readonly.
	OnResponse(resp *http.Response, clientCert *x509.Certificate)
}

var (
	registeredPlugins = make(map[string]Plugin)
	enabledPlugins    []string
)

// Register is for plugins to register them.
func Register(p Plugin) {
	registeredPlugins[p.Name()] = p
}

func BindCmdFlags(cmd *cobra.Command) {
	names := make([]string, 0, len(registeredPlugins))
	for name := range registeredPlugins {
		names = append(names, name)
	}
	cmd.Flags().StringArrayVar(&enabledPlugins, "plugins", nil, "enable plugins ("+strings.Join(names, ",")+")")
}

func Setup() {
	for _, name := range enabledPlugins {
		if p, exists := registeredPlugins[name]; exists {
			p.Setup()
		} else {
			logger.L.Warn("unrecognized plugin", zap.String("name", name))
		}
	}
}

func OnRequest(req *http.Request, clientCert *x509.Certificate) {
	for _, name := range enabledPlugins {
		if p, exists := registeredPlugins[name]; exists {
			go p.OnRequest(req, clientCert)
		}
	}
}

func OnResponse(resp *http.Response, clientCert *x509.Certificate) {
	wg := sync.WaitGroup{}
	for _, name := range enabledPlugins {
		if p, exists := registeredPlugins[name]; exists {
			wg.Add(1)
			go func() {
				p.OnResponse(resp, clientCert)
				wg.Done()
			}()
		}
	}
	go func() {
		wg.Wait()
		_ = resp.Body.Close()
	}()
}
