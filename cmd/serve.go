package cmd

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/prasenjit-net/mcp-gateway/admin"
	"github.com/prasenjit-net/mcp-gateway/auth"
	"github.com/prasenjit-net/mcp-gateway/config"
	"github.com/prasenjit-net/mcp-gateway/mcp"
	"github.com/prasenjit-net/mcp-gateway/proxy"
	"github.com/prasenjit-net/mcp-gateway/registry"
	"github.com/prasenjit-net/mcp-gateway/store"
	"github.com/prasenjit-net/mcp-gateway/telemetry"
	"github.com/prasenjit-net/mcp-gateway/tlsutil"
	"github.com/soheilhy/cmux"
)

func runServe(args []string, opts Options) {
	fs := newFlagSet("serve")
	configFile := fs.String("config", config.DefaultConfigFile, "path to config file")
	port := fs.String("port", "", "listen port, overrides listen_addr in config (e.g. 9090)")
	fs.StringVar(port, "P", "", "listen port (shorthand)")
	tlsFlag := fs.Bool("tls", false, "enable TLS (overrides tls.enabled in config)")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: mcp-gateway serve [options]

Start the MCP Gateway HTTP server.

Options:
`)
		fs.PrintDefaults()
	}
	fs.Parse(args) //nolint:errcheck

	cfg, err := config.Load(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	// --port / -P overrides whatever is in the config file.
	if *port != "" {
		cfg.ListenAddr = ":" + *port
	}
	// --tls flag overrides tls.enabled in config.
	if *tlsFlag {
		cfg.TLS.Enabled = true
	}

	logger := telemetry.Setup(cfg.LogLevel)
	logger.Info("starting mcp-gateway",
		"config", *configFile,
		"addr", cfg.ListenAddr,
		"data_dir", cfg.DataDir,
	)

	telemetry.Register()

	jsonStore, err := store.NewJSONStore(cfg.DataDir)
	if err != nil {
		logger.Error("failed to create store", "error", err)
		os.Exit(1)
	}

	reg := registry.NewRegistry()
	admin.RebuildRegistryFromStore(jsonStore, reg)

	// Registry subscription goroutine: exits when ctx is cancelled on shutdown.
	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	regCh := reg.Subscribe()
	go func() {
		defer reg.Unsubscribe(regCh)
		for {
			select {
			case <-shutdownCtx.Done():
				return
			case _, ok := <-regCh:
				if !ok {
					return
				}
				tools := reg.List()
				telemetry.RegistryToolsTotal.Set(float64(len(tools)))
			}
		}
	}()

	// Build mTLS config if cert/key exist on disk.
	var mtlsCfg *tls.Config
	if _, certErr := os.Stat(cfg.MTLS.CertFile); certErr == nil {
		if _, keyErr := os.Stat(cfg.MTLS.KeyFile); keyErr == nil {
			mc, err := tlsutil.NewMTLSClientConfig(cfg.MTLS.CertFile, cfg.MTLS.KeyFile, cfg.MTLS.CAFile)
			if err != nil {
				logger.Error("failed to load mTLS config", "error", err)
			} else {
				mtlsCfg = mc
				logger.Info("mTLS client certificate loaded", "cert", cfg.MTLS.CertFile)
			}
		}
	}

	proxyClient := proxy.NewProxyWithMTLS(30*time.Second, mtlsCfg)
	deps := &mcp.HandlerDeps{
		Registry:       reg,
		Proxy:          proxyClient,
		Store:          jsonStore,
		Config:         cfg,
		Authenticators: make(map[string]auth.Authenticator),
	}

	sseServer := mcp.NewSSEServer(deps)
	httpTransport := mcp.NewHTTPTransport(deps)

	mux := http.NewServeMux()
	adminDeps := &admin.Deps{
		Store:    jsonStore,
		Registry: reg,
		SSE:      sseServer,
		HTTP:     httpTransport,
		Config:   cfg,
	}
	admin.RegisterRoutes(mux, adminDeps)

	if cfg.UIDevProxy != "" {
		target, err := url.Parse(cfg.UIDevProxy)
		if err != nil {
			logger.Error("invalid UI_DEV_PROXY URL", "error", err)
			os.Exit(1)
		}
		rp := httputil.NewSingleHostReverseProxy(target)
		mux.HandleFunc("GET /_ui/", admin.UIAuthMiddleware(cfg, func(w http.ResponseWriter, r *http.Request) {
			rp.ServeHTTP(w, r)
		}))
	} else if opts.UIHandler != nil {
		mux.Handle("GET /_ui/", admin.UIAuthMiddleware(cfg, opts.UIHandler().ServeHTTP))
	}

	server := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: mux,
	}

	// cmuxInst holds the multiplexer when TLS mode is active so we can close it on shutdown.
	var cmuxInst cmux.CMux

	if cfg.TLS.Enabled {
		cert, err := tlsutil.LoadTLSCertificate(cfg.TLS.CertFile, cfg.TLS.KeyFile)
		if err != nil {
			logger.Error("failed to load TLS certificate", "error", err)
			os.Exit(1)
		}
		tlsCfg := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}

		ln, err := net.Listen("tcp", cfg.ListenAddr)
		if err != nil {
			logger.Error("failed to listen", "addr", cfg.ListenAddr, "error", err)
			os.Exit(1)
		}

		m := cmux.New(ln)
		cmuxInst = m
		tlsL := m.Match(cmux.TLS())
		httpL := m.Match(cmux.Any())

		tlsListener := tls.NewListener(tlsL, tlsCfg)

		var cmuxWG sync.WaitGroup
		cmuxWG.Add(3)
		go func() {
			defer cmuxWG.Done()
			if err := server.Serve(tlsListener); err != nil && err != http.ErrServerClosed {
				logger.Error("TLS server error", "error", err)
			}
		}()
		go func() {
			defer cmuxWG.Done()
			if err := server.Serve(httpL); err != nil && err != http.ErrServerClosed {
				logger.Error("HTTP server error", "error", err)
			}
		}()
		go func() {
			defer cmuxWG.Done()
			if err := m.Serve(); err != nil {
				logger.Error("cmux error", "error", err)
			}
		}()
		// Store WG on the server so shutdown can wait for goroutines.
		_ = cmuxWG // goroutines drain when cmuxInst.Close() is called below

		logger.Info("TLS enabled on addr (HTTP and HTTPS)", "addr", cfg.ListenAddr)
	} else {
		go func() {
			logger.Info("listening", "addr", cfg.ListenAddr)
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("server error", "error", err)
				os.Exit(1)
			}
		}()
	}

	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("shutting down...")
	shutdownCancel() // stop registry subscription goroutine

	httpCtx, httpCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer httpCancel()

	if err := server.Shutdown(httpCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
	// Close cmux after HTTP server drains so sub-listeners are also closed.
	if cmuxInst != nil {
		cmuxInst.Close()
	}
	jsonStore.Close()
	logger.Info("shutdown complete")
}
