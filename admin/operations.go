package admin

import (
	"encoding/json"
	"net/http"

	"mcp-gateway/config"
	"mcp-gateway/registry"
	"mcp-gateway/store"
)

type opsHandler struct {
	store    store.Store
	registry *registry.Registry
	config   *config.Config
}

func (h *opsHandler) UpdateOperation(w http.ResponseWriter, r *http.Request) {
	specID := r.PathValue("id")
	opID := r.PathValue("opId")

	ops, err := h.store.GetOperations(specID)
	if err != nil {
		jsonError(w, "spec not found", http.StatusNotFound)
		return
	}

	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	var target *store.OperationRecord
	for _, op := range ops {
		if op.ID == opID {
			target = op
			break
		}
	}
	if target == nil {
		jsonError(w, "operation not found", http.StatusNotFound)
		return
	}

	if body.Enabled != nil {
		target.Enabled = *body.Enabled
	}

	if err := h.store.UpdateOperation(specID, target); err != nil {
		jsonError(w, "failed to update operation: "+err.Error(), http.StatusInternalServerError)
		return
	}

	RebuildRegistryFromStore(h.store, h.registry)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(target)
}
