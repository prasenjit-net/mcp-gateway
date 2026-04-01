package mcp

import (
	"encoding/json"
	"io"
	"net/http"
)

type HTTPTransport struct {
	deps *HandlerDeps
}

func NewHTTPTransport(deps *HandlerDeps) *HTTPTransport {
	return &HTTPTransport{deps: deps}
}

func (t *HTTPTransport) Handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	inbound := captureInboundAuth(r)

	defer r.Body.Close()

	limitedBody := io.LimitReader(r.Body, t.deps.Config.MaxRequestBytes)
	var raw json.RawMessage
	if err := json.NewDecoder(limitedBody).Decode(&raw); err != nil {
		http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
		return
	}

	if len(raw) > 0 && raw[0] == '[' {
		var reqs []Request
		if err := json.Unmarshal(raw, &reqs); err != nil {
			http.Error(w, `{"error":"invalid batch JSON"}`, http.StatusBadRequest)
			return
		}
		responses := make([]*Response, 0, len(reqs))
		for i := range reqs {
			resp := t.deps.Handle(r.Context(), &reqs[i], inbound)
			if resp != nil {
				responses = append(responses, resp)
			}
		}
		json.NewEncoder(w).Encode(responses)
	} else {
		var req Request
		if err := json.Unmarshal(raw, &req); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}
		resp := t.deps.Handle(r.Context(), &req, inbound)
		if resp == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		json.NewEncoder(w).Encode(resp)
	}
}
