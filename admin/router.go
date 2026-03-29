package admin

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prasenjit-net/mcp-gateway/config"
	"github.com/prasenjit-net/mcp-gateway/mcp"
	"github.com/prasenjit-net/mcp-gateway/registry"
	"github.com/prasenjit-net/mcp-gateway/store"
)

type Deps struct {
	Store    store.Store
	Registry *registry.Registry
	SSE      *mcp.SSEServer
	HTTP     *mcp.HTTPTransport
	Config   *config.Config
}

func RegisterRoutes(mux *http.ServeMux, deps *Deps) {
	mux.HandleFunc("GET /mcp/sse", corsMiddleware(deps.SSE.HandleSSE))
	mux.HandleFunc("POST /mcp/sse/message", corsMiddleware(deps.SSE.HandleMessage))
	mux.HandleFunc("POST /mcp/http", corsMiddleware(deps.HTTP.Handle))

	mux.Handle("GET /metrics", promhttp.Handler())

	specHandler := &specsHandler{store: deps.Store, registry: deps.Registry, config: deps.Config}
	mux.HandleFunc("POST /_api/specs", corsMiddleware(specHandler.Create))
	mux.HandleFunc("GET /_api/specs", corsMiddleware(specHandler.List))
	mux.HandleFunc("GET /_api/specs/{id}", corsMiddleware(specHandler.Get))
	mux.HandleFunc("PATCH /_api/specs/{id}", corsMiddleware(specHandler.Update))
	mux.HandleFunc("DELETE /_api/specs/{id}", corsMiddleware(specHandler.Delete))
	mux.HandleFunc("GET /_api/specs/{id}/operations", corsMiddleware(specHandler.ListOperations))

	opsHandler := &opsHandler{store: deps.Store, registry: deps.Registry, config: deps.Config}
	mux.HandleFunc("PATCH /_api/specs/{id}/operations/{opId}", corsMiddleware(opsHandler.UpdateOperation))

	statsHdlr := &statsHandler{store: deps.Store, registry: deps.Registry, sse: deps.SSE}
	mux.HandleFunc("GET /_api/stats", corsMiddleware(statsHdlr.Stats))
	mux.HandleFunc("GET /_api/stats/tools", corsMiddleware(statsHdlr.ToolStats))
	mux.HandleFunc("GET /_api/health", corsMiddleware(statsHdlr.Health))

	mux.HandleFunc("OPTIONS /", corsPreflightHandler)
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		next(w, r)
	}
}

func corsPreflightHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.WriteHeader(http.StatusNoContent)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write([]byte(`{"error":"` + msg + `"}`))
}
