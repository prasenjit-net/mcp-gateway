package admin

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/prasenjit-net/mcp-gateway/config"
	"github.com/prasenjit-net/mcp-gateway/registry"
	"github.com/prasenjit-net/mcp-gateway/spec"
	"github.com/prasenjit-net/mcp-gateway/store"
)

type specsHandler struct {
	store    store.Store
	registry *registry.Registry
	config   *config.Config
}

func (h *specsHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		var body struct {
			Name               string   `json:"name"`
			UpstreamURL        string   `json:"upstream_url"`
			SpecRaw            string   `json:"spec_raw"`
			PassthroughAuth    bool     `json:"passthrough_auth"`
			PassthroughCookies bool     `json:"passthrough_cookies"`
			PassthroughHeaders []string `json:"passthrough_headers"`
			MTLSEnabled        bool     `json:"mtls_enabled"`
		}
		if err2 := json.NewDecoder(r.Body).Decode(&body); err2 != nil {
			jsonError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		h.createFromRaw(w, body.Name, body.UpstreamURL, []byte(body.SpecRaw), body.PassthroughAuth, body.PassthroughCookies, body.PassthroughHeaders, body.MTLSEnabled)
		return
	}

	name := r.FormValue("name")
	upstreamURL := r.FormValue("upstream_url")
	passthroughAuth := r.FormValue("passthrough_auth") == "true"
	passthroughCookies := r.FormValue("passthrough_cookies") == "true"
	mtlsEnabled := r.FormValue("mtls_enabled") == "true"
	var passthroughHeaders []string
	if ph := r.FormValue("passthrough_headers"); ph != "" {
		json.Unmarshal([]byte(ph), &passthroughHeaders)
	}

	var specData []byte
	if f, _, err := r.FormFile("spec"); err == nil {
		defer f.Close()
		var buf []byte
		tmp := make([]byte, 4096)
		for {
			n, err := f.Read(tmp)
			if n > 0 {
				buf = append(buf, tmp[:n]...)
			}
			if err != nil {
				break
			}
		}
		specData = buf
	} else {
		specData = []byte(r.FormValue("spec_raw"))
	}

	h.createFromRaw(w, name, upstreamURL, specData, passthroughAuth, passthroughCookies, passthroughHeaders, mtlsEnabled)
}

func (h *specsHandler) createFromRaw(w http.ResponseWriter, name, upstreamURL string, specData []byte, passthroughAuth, passthroughCookies bool, passthroughHeaders []string, mtlsEnabled bool) {
	if len(specData) == 0 {
		jsonError(w, "spec_raw or spec file required", http.StatusBadRequest)
		return
	}

	parsed, err := spec.Parse(specData)
	if err != nil {
		jsonError(w, "invalid spec: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Auto-fill name from info.title when not provided
	if name == "" && parsed.Doc.Info != nil {
		name = parsed.Doc.Info.Title
	}
	// Auto-fill upstream from first servers entry when not provided
	if upstreamURL == "" && len(parsed.Doc.Servers) > 0 {
		upstreamURL = parsed.Doc.Servers[0].URL
	}

	id := uuid.New().String()
	now := time.Now()
	rec := &store.SpecRecord{
		ID:                 id,
		Name:               name,
		UpstreamURL:        upstreamURL,
		SpecRaw:            string(specData),
		PassthroughAuth:    passthroughAuth,
		PassthroughCookies: passthroughCookies,
		PassthroughHeaders: passthroughHeaders,
		MTLSEnabled:        mtlsEnabled,
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	_, ops, err := spec.ExtractTools(id, name, upstreamURL, parsed, passthroughAuth, passthroughCookies, passthroughHeaders, mtlsEnabled)
	if err != nil {
		jsonError(w, "failed to extract tools: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := h.store.SaveSpec(rec); err != nil {
		jsonError(w, "failed to save spec: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := h.store.SaveOperations(id, ops); err != nil {
		jsonError(w, "failed to save operations: "+err.Error(), http.StatusInternalServerError)
		return
	}

	RebuildRegistryFromStore(h.store, h.registry)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(rec)
}

func (h *specsHandler) List(w http.ResponseWriter, r *http.Request) {
	specs, err := h.store.ListSpecs()
	if err != nil {
		jsonError(w, "failed to list specs: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if specs == nil {
		specs = []*store.SpecRecord{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(specs)
}

func (h *specsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rec, err := h.store.GetSpec(id)
	if err != nil {
		jsonError(w, "spec not found", http.StatusNotFound)
		return
	}
	ops, _ := h.store.GetOperations(id)

	result := map[string]interface{}{
		"spec":       rec,
		"operations": ops,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *specsHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rec, err := h.store.GetSpec(id)
	if err != nil {
		jsonError(w, "spec not found", http.StatusNotFound)
		return
	}

	var body struct {
		Name               *string  `json:"name"`
		UpstreamURL        *string  `json:"upstream_url"`
		PassthroughAuth    *bool    `json:"passthrough_auth"`
		PassthroughCookies *bool    `json:"passthrough_cookies"`
		PassthroughHeaders []string `json:"passthrough_headers"`
		MTLSEnabled        *bool    `json:"mtls_enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if body.Name != nil {
		rec.Name = *body.Name
	}
	if body.UpstreamURL != nil {
		rec.UpstreamURL = *body.UpstreamURL
	}
	if body.PassthroughAuth != nil {
		rec.PassthroughAuth = *body.PassthroughAuth
	}
	if body.PassthroughCookies != nil {
		rec.PassthroughCookies = *body.PassthroughCookies
	}
	if body.PassthroughHeaders != nil {
		rec.PassthroughHeaders = body.PassthroughHeaders
	}
	if body.MTLSEnabled != nil {
		rec.MTLSEnabled = *body.MTLSEnabled
	}
	rec.UpdatedAt = time.Now()

	if err := h.store.SaveSpec(rec); err != nil {
		jsonError(w, "failed to save spec: "+err.Error(), http.StatusInternalServerError)
		return
	}

	RebuildRegistryFromStore(h.store, h.registry)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rec)
}

func (h *specsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := h.store.GetSpec(id); err != nil {
		jsonError(w, "spec not found", http.StatusNotFound)
		return
	}

	h.store.DeleteSpec(id)
	h.store.DeleteOperations(id)
	h.store.DeleteAuth(id)

	RebuildRegistryFromStore(h.store, h.registry)

	w.WriteHeader(http.StatusNoContent)
}

func (h *specsHandler) ListOperations(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ops, err := h.store.GetOperations(id)
	if err != nil {
		jsonError(w, "spec not found or no operations", http.StatusNotFound)
		return
	}
	if ops == nil {
		ops = []*store.OperationRecord{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ops)
}

// RebuildRegistryFromStore rebuilds the tool registry from all stored specs.
func RebuildRegistryFromStore(st store.Store, reg *registry.Registry) {
	specs, err := st.ListSpecs()
	if err != nil {
		return
	}

	var allTools []*spec.ToolDefinition
	for _, specRec := range specs {
		ops, err := st.GetOperations(specRec.ID)
		if err != nil {
			continue
		}

		parsed, err := spec.Parse([]byte(specRec.SpecRaw))
		if err != nil {
			continue
		}

		tools, _, err := spec.ExtractTools(
			specRec.ID,
			specRec.Name,
			specRec.UpstreamURL,
			parsed,
			specRec.PassthroughAuth,
			specRec.PassthroughCookies,
			specRec.PassthroughHeaders,
			specRec.MTLSEnabled,
		)
		if err != nil {
			continue
		}

		enabledOps := map[string]bool{}
		for _, op := range ops {
			if op.Enabled {
				enabledOps[op.OperationID] = true
			}
		}

		for _, t := range tools {
			if enabledOps[t.OperationID] {
				allTools = append(allTools, t)
			}
		}
	}

	reg.RebuildAll(allTools)
}
