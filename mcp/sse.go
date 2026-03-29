package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"mcp-gateway/auth"
)

type SSEServer struct {
	deps     *HandlerDeps
	sessions sync.Map
}

type sseSession struct {
	id      string
	outCh   chan []byte
	inbound *auth.InboundAuth
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewSSEServer(deps *HandlerDeps) *SSEServer {
	return &SSEServer{deps: deps}
}

func (s *SSEServer) HandleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	inbound := captureInboundAuth(r)

	sessionID := uuid.New().String()
	ctx, cancel := context.WithCancel(r.Context())
	session := &sseSession{
		id:      sessionID,
		outCh:   make(chan []byte, 64),
		inbound: inbound,
		ctx:     ctx,
		cancel:  cancel,
	}
	s.sessions.Store(sessionID, session)
	defer func() {
		s.sessions.Delete(sessionID)
		cancel()
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	endpointURL := fmt.Sprintf("/mcp/sse/message?sessionId=%s", sessionID)
	fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", endpointURL)
	flusher.Flush()

	regCh := s.deps.Registry.Subscribe()
	defer s.deps.Registry.Unsubscribe(regCh)

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()
		case <-regCh:
			notification := Notification{
				JSONRPC: "2.0",
				Method:  "notifications/tools/list_changed",
			}
			data, _ := json.Marshal(notification)
			fmt.Fprintf(w, "event: notification\ndata: %s\n\n", string(data))
			flusher.Flush()
		case msg := <-session.outCh:
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", string(msg))
			flusher.Flush()
		}
	}
}

func (s *SSEServer) HandleMessage(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("sessionId")
	if sessionID == "" {
		http.Error(w, "missing sessionId", http.StatusBadRequest)
		return
	}

	val, ok := s.sessions.Load(sessionID)
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	session := val.(*sseSession)

	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	resp := s.deps.Handle(session.ctx, &req, session.inbound)
	if resp == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	data, err := json.Marshal(resp)
	if err != nil {
		slog.Error("failed to marshal response", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	select {
	case session.outCh <- data:
	case <-time.After(200 * time.Millisecond):
		slog.Warn("session outCh full, dropping response", "sessionId", sessionID)
	}

	w.WriteHeader(http.StatusAccepted)
}

func (s *SSEServer) ActiveSessionCount() int {
	count := 0
	s.sessions.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

func captureInboundAuth(r *http.Request) *auth.InboundAuth {
	return &auth.InboundAuth{
		Authorization: r.Header.Get("Authorization"),
		Cookie:        r.Header.Get("Cookie"),
		ExtraHeaders:  map[string]string{},
	}
}
