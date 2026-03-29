package admin

import (
	"encoding/json"
	"net/http"
	"time"

	"mcp-gateway/mcp"
	"mcp-gateway/registry"
	"mcp-gateway/store"
)

var startTime = time.Now()

type statsHandler struct {
	store    store.Store
	registry *registry.Registry
	sse      *mcp.SSEServer
}

func (h *statsHandler) Stats(w http.ResponseWriter, r *http.Request) {
	specs, _ := h.store.ListSpecs()
	tools := h.registry.List()
	allStats, _ := h.store.GetAllStats()

	var totalCalls, totalErrors int64
	for _, s := range allStats {
		totalCalls += s.CallCount
		totalErrors += s.ErrorCount
	}

	totalSpecs := 0
	if specs != nil {
		totalSpecs = len(specs)
	}

	result := map[string]interface{}{
		"totalSpecs":     totalSpecs,
		"totalTools":     len(tools),
		"enabledTools":   len(tools),
		"totalCalls":     totalCalls,
		"totalErrors":    totalErrors,
		"activeSessions": h.sse.ActiveSessionCount(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *statsHandler) ToolStats(w http.ResponseWriter, r *http.Request) {
	allStats, err := h.store.GetAllStats()
	if err != nil {
		jsonError(w, "failed to get stats", http.StatusInternalServerError)
		return
	}

	result := make([]interface{}, 0, len(allStats))
	for _, s := range allStats {
		result = append(result, s)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *statsHandler) Health(w http.ResponseWriter, r *http.Request) {
	uptime := time.Since(startTime)
	result := map[string]interface{}{
		"status":  "ok",
		"uptime":  uptime.String(),
		"version": "0.1.0",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
