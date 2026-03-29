package main

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
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
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger := telemetry.Setup(cfg.LogLevel)
	logger.Info("starting mcp-gateway", "addr", cfg.ListenAddr, "data_dir", cfg.DataDir)

	telemetry.Register()

	jsonStore, err := store.NewJSONStore(cfg.DataDir)
	if err != nil {
		logger.Error("failed to create store", "error", err)
		os.Exit(1)
	}

	reg := registry.NewRegistry()
	admin.RebuildRegistryFromStore(jsonStore, reg)

	regCh := reg.Subscribe()
	go func() {
		for range regCh {
			tools := reg.List()
			telemetry.RegistryToolsTotal.Set(float64(len(tools)))
		}
	}()

	proxyClient := proxy.NewProxy(30 * time.Second)
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
		mux.HandleFunc("GET /_ui/", func(w http.ResponseWriter, r *http.Request) {
			rp.ServeHTTP(w, r)
		})
	} else {
		mux.Handle("GET /_ui/", uiHandler())
	}

	server := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: mux,
	}

	go func() {
		logger.Info("listening", "addr", cfg.ListenAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	logger.Info("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	server.Shutdown(ctx)
	jsonStore.Close()
	logger.Info("shutdown complete")
}
