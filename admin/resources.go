package admin

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/prasenjit-net/mcp-gateway/config"
	"github.com/prasenjit-net/mcp-gateway/registry"
	"github.com/prasenjit-net/mcp-gateway/store"
)

type resourcesHandler struct {
	store    store.Store
	registry *registry.Registry
	config   *config.Config
}

func (h *resourcesHandler) Create(w http.ResponseWriter, r *http.Request) {
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "multipart/form-data") {
		h.createFile(w, r)
	} else {
		h.createJSON(w, r)
	}
}

func (h *resourcesHandler) createFile(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		jsonError(w, "invalid multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	if name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}
	description := r.FormValue("description")
	mimeType := r.FormValue("mime_type")

	f, fh, err := r.FormFile("file")
	if err != nil {
		jsonError(w, "file is required: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer f.Close()

	id := uuid.New().String()
	relDir := filepath.Join("resources", id)
	absDir := filepath.Join(h.config.DataDir, relDir)
	if err := os.MkdirAll(absDir, 0o750); err != nil {
		jsonError(w, "failed to create storage dir: "+err.Error(), http.StatusInternalServerError)
		return
	}

	filename := sanitizeFilename(fh.Filename)
	relPath := filepath.Join(relDir, filename)
	absPath, err := store.SafeJoin(h.config.DataDir, relPath)
	if err != nil {
		jsonError(w, "invalid file path", http.StatusBadRequest)
		return
	}

	out, err := os.OpenFile(absPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		jsonError(w, "failed to create file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer out.Close()
	if _, err := io.Copy(out, f); err != nil {
		jsonError(w, "failed to write file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if mimeType == "" {
		mimeType = detectMimeType(filename)
	}

	now := time.Now()
	rec := &store.ResourceRecord{
		ID:          id,
		Name:        name,
		Description: description,
		Type:        "file",
		MimeType:    mimeType,
		FilePath:    relPath,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := h.store.SaveResource(rec); err != nil {
		jsonError(w, "failed to save resource: "+err.Error(), http.StatusInternalServerError)
		return
	}
	rebuildResources(h.store, h.registry)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(rec)
}

func (h *resourcesHandler) createJSON(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name               string   `json:"name"`
		Description        string   `json:"description"`
		Type               string   `json:"type"`
		MimeType           string   `json:"mime_type"`
		Content            string   `json:"content"`
		UpstreamURL        string   `json:"upstream_url"`
		URITemplate        string   `json:"uri_template"`
		PassthroughAuth    bool     `json:"passthrough_auth"`
		PassthroughCookies bool     `json:"passthrough_cookies"`
		PassthroughHeaders []string `json:"passthrough_headers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if body.Name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}

	id := uuid.New().String()
	now := time.Now()
	rec := &store.ResourceRecord{
		ID:          id,
		Name:        body.Name,
		Description: body.Description,
		Type:        body.Type,
		MimeType:    body.MimeType,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	switch body.Type {
	case "text":
		if body.MimeType == "" {
			rec.MimeType = "text/plain"
		}
		relDir := filepath.Join("resources", id)
		absDir := filepath.Join(h.config.DataDir, relDir)
		if err := os.MkdirAll(absDir, 0o750); err != nil {
			jsonError(w, "failed to create storage dir: "+err.Error(), http.StatusInternalServerError)
			return
		}
		relPath := filepath.Join(relDir, "content.txt")
		absPath, err := store.SafeJoin(h.config.DataDir, relPath)
		if err != nil {
			jsonError(w, "invalid content path", http.StatusInternalServerError)
			return
		}
		if err := os.WriteFile(absPath, []byte(body.Content), 0o600); err != nil {
			jsonError(w, "failed to write content: "+err.Error(), http.StatusInternalServerError)
			return
		}
		rec.FilePath = relPath

	case "upstream":
		if body.UpstreamURL == "" {
			jsonError(w, "upstream_url is required for upstream type", http.StatusBadRequest)
			return
		}
		isTemplate := strings.Contains(body.UpstreamURL, "{")
		rec.UpstreamURL = body.UpstreamURL
		rec.IsTemplate = isTemplate
		rec.URITemplate = body.URITemplate
		rec.PassthroughAuth = body.PassthroughAuth
		rec.PassthroughCookies = body.PassthroughCookies
		rec.PassthroughHeaders = body.PassthroughHeaders
		if body.MimeType == "" {
			rec.MimeType = "application/json"
		}
		if isTemplate && rec.URITemplate == "" {
			rec.URITemplate = "gateway://resources/" + id
		}

	default:
		jsonError(w, "type must be 'text' or 'upstream'", http.StatusBadRequest)
		return
	}

	if err := h.store.SaveResource(rec); err != nil {
		jsonError(w, "failed to save resource: "+err.Error(), http.StatusInternalServerError)
		return
	}
	rebuildResources(h.store, h.registry)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(rec)
}

func (h *resourcesHandler) List(w http.ResponseWriter, r *http.Request) {
	resources, err := h.store.ListResources()
	if err != nil {
		jsonError(w, "failed to list resources: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if resources == nil {
		resources = []*store.ResourceRecord{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resources)
}

func (h *resourcesHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rec, err := h.store.GetResource(id)
	if err != nil {
		jsonError(w, "resource not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rec)
}

func (h *resourcesHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rec, err := h.store.GetResource(id)
	if err != nil {
		jsonError(w, "resource not found", http.StatusNotFound)
		return
	}
	var body struct {
		Name               *string  `json:"name"`
		Description        *string  `json:"description"`
		MimeType           *string  `json:"mime_type"`
		UpstreamURL        *string  `json:"upstream_url"`
		URITemplate        *string  `json:"uri_template"`
		PassthroughAuth    *bool    `json:"passthrough_auth"`
		PassthroughCookies *bool    `json:"passthrough_cookies"`
		PassthroughHeaders []string `json:"passthrough_headers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if body.Name != nil {
		rec.Name = *body.Name
	}
	if body.Description != nil {
		rec.Description = *body.Description
	}
	if body.MimeType != nil {
		rec.MimeType = *body.MimeType
	}
	if body.UpstreamURL != nil {
		rec.UpstreamURL = *body.UpstreamURL
		rec.IsTemplate = strings.Contains(*body.UpstreamURL, "{")
	}
	if body.URITemplate != nil {
		rec.URITemplate = *body.URITemplate
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
	rec.UpdatedAt = time.Now()
	if err := h.store.SaveResource(rec); err != nil {
		jsonError(w, "failed to save resource: "+err.Error(), http.StatusInternalServerError)
		return
	}
	rebuildResources(h.store, h.registry)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rec)
}

func (h *resourcesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := h.store.GetResource(id); err != nil {
		jsonError(w, "resource not found", http.StatusNotFound)
		return
	}
	if err := h.store.DeleteResource(id); err != nil {
		jsonError(w, "failed to delete resource: "+err.Error(), http.StatusInternalServerError)
		return
	}
	rebuildResources(h.store, h.registry)
	w.WriteHeader(http.StatusNoContent)
}

func (h *resourcesHandler) GetContent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rec, err := h.store.GetResource(id)
	if err != nil {
		jsonError(w, "resource not found", http.StatusNotFound)
		return
	}
	if rec.Type == "upstream" {
		jsonError(w, "upstream resources have no stored content", http.StatusBadRequest)
		return
	}
	absPath, err := store.SafeJoin(h.config.DataDir, rec.FilePath)
	if err != nil {
		jsonError(w, "invalid resource path", http.StatusInternalServerError)
		return
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		jsonError(w, "failed to read content: "+err.Error(), http.StatusInternalServerError)
		return
	}
	mime := rec.MimeType
	if mime == "" {
		mime = "application/octet-stream"
	}
	w.Header().Set("Content-Type", mime)
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

func rebuildResources(st store.Store, reg *registry.Registry) {
	resources, err := st.ListResources()
	if err != nil {
		return
	}
	reg.RebuildResources(resources)
}

func sanitizeFilename(name string) string {
	var out []byte
	for _, c := range []byte(name) {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '.' || c == '-' || c == '_' {
			out = append(out, c)
		} else {
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return "file"
	}
	return string(out)
}

func detectMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".json":
		return "application/json"
	case ".yaml", ".yml":
		return "application/yaml"
	case ".md":
		return "text/markdown"
	case ".txt":
		return "text/plain"
	case ".html":
		return "text/html"
	case ".xml":
		return "application/xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".pdf":
		return "application/pdf"
	case ".csv":
		return "text/csv"
	default:
		return "application/octet-stream"
	}
}
