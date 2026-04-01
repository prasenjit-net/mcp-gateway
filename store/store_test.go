package store_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/prasenjit-net/mcp-gateway/store"
)

func newStore(t *testing.T) *store.JSONStore {
	t.Helper()
	s, err := store.NewJSONStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSONStore: %v", err)
	}
	t.Cleanup(func() { s.Close() }) //nolint:errcheck
	return s
}

// ── Specs ─────────────────────────────────────────────────────────────────────

func TestSaveAndGetSpec(t *testing.T) {
	s := newStore(t)
	rec := &store.SpecRecord{
		ID:          "spec-1",
		Name:        "Test Spec",
		UpstreamURL: "https://api.example.com",
		SpecRaw:     `{"openapi":"3.0.0"}`,
		CreatedAt:   time.Now(),
	}
	if err := s.SaveSpec(rec); err != nil {
		t.Fatalf("SaveSpec: %v", err)
	}
	got, err := s.GetSpec("spec-1")
	if err != nil {
		t.Fatalf("GetSpec: %v", err)
	}
	if got.Name != "Test Spec" {
		t.Errorf("Name = %q, want Test Spec", got.Name)
	}
	if got.UpstreamURL != "https://api.example.com" {
		t.Errorf("UpstreamURL = %q", got.UpstreamURL)
	}
}

func TestGetSpecNotFound(t *testing.T) {
	s := newStore(t)
	_, err := s.GetSpec("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent spec")
	}
}

func TestListSpecs(t *testing.T) {
	s := newStore(t)
	for _, id := range []string{"s1", "s2", "s3"} {
		if err := s.SaveSpec(&store.SpecRecord{ID: id, Name: id}); err != nil {
			t.Fatal(err)
		}
	}
	specs, err := s.ListSpecs()
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 3 {
		t.Errorf("ListSpecs() = %d, want 3", len(specs))
	}
}

func TestDeleteSpec(t *testing.T) {
	s := newStore(t)
	if err := s.SaveSpec(&store.SpecRecord{ID: "del-spec"}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteSpec("del-spec"); err != nil {
		t.Fatalf("DeleteSpec: %v", err)
	}
	_, err := s.GetSpec("del-spec")
	if err == nil {
		t.Error("spec should have been deleted")
	}
}

// ── Operations ────────────────────────────────────────────────────────────────

func TestSaveAndGetOperations(t *testing.T) {
	s := newStore(t)
	ops := []*store.OperationRecord{
		{ID: "op-1", SpecID: "spec-1", OperationID: "getUser", Enabled: true},
		{ID: "op-2", SpecID: "spec-1", OperationID: "listUsers", Enabled: false},
	}
	if err := s.SaveOperations("spec-1", ops); err != nil {
		t.Fatalf("SaveOperations: %v", err)
	}
	got, err := s.GetOperations("spec-1")
	if err != nil {
		t.Fatalf("GetOperations: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("GetOperations() = %d ops, want 2", len(got))
	}
}

func TestUpdateOperation(t *testing.T) {
	s := newStore(t)
	ops := []*store.OperationRecord{
		{ID: "op-1", SpecID: "spec-1", Enabled: true},
	}
	s.SaveOperations("spec-1", ops) //nolint:errcheck

	updated := &store.OperationRecord{ID: "op-1", SpecID: "spec-1", Enabled: false}
	if err := s.UpdateOperation("spec-1", updated); err != nil {
		t.Fatalf("UpdateOperation: %v", err)
	}
	got, _ := s.GetOperations("spec-1")
	if got[0].Enabled {
		t.Error("operation should be disabled after update")
	}
}

func TestUpdateOperationNotFound(t *testing.T) {
	s := newStore(t)
	s.SaveOperations("spec-1", []*store.OperationRecord{{ID: "op-1", SpecID: "spec-1"}}) //nolint:errcheck
	err := s.UpdateOperation("spec-1", &store.OperationRecord{ID: "no-such-op"})
	if err == nil {
		t.Error("expected error for unknown operation ID")
	}
}

func TestDeleteOperations(t *testing.T) {
	s := newStore(t)
	s.SaveOperations("spec-1", []*store.OperationRecord{{ID: "op-1"}}) //nolint:errcheck
	if err := s.DeleteOperations("spec-1"); err != nil {
		t.Fatalf("DeleteOperations: %v", err)
	}
	_, err := s.GetOperations("spec-1")
	if err == nil {
		t.Error("expected error after deleting operations")
	}
}

// ── Auth ──────────────────────────────────────────────────────────────────────

func TestSaveAndGetAuth(t *testing.T) {
	s := newStore(t)
	cfg := &store.AuthConfig{
		Type:   "bearer",
		Config: json.RawMessage(`{"token":"test-token"}`),
	}
	if err := s.SaveAuth("spec-1", cfg); err != nil {
		t.Fatalf("SaveAuth: %v", err)
	}
	got, err := s.GetAuth("spec-1")
	if err != nil {
		t.Fatalf("GetAuth: %v", err)
	}
	if got.Type != "bearer" {
		t.Errorf("Type = %q, want bearer", got.Type)
	}
}

func TestDeleteAuth(t *testing.T) {
	s := newStore(t)
	s.SaveAuth("spec-1", &store.AuthConfig{Type: "bearer", Config: json.RawMessage(`{}`)}) //nolint:errcheck
	if err := s.DeleteAuth("spec-1"); err != nil {
		t.Fatalf("DeleteAuth: %v", err)
	}
	// Delete of nonexistent should not error
	if err := s.DeleteAuth("spec-1"); err != nil {
		t.Errorf("second DeleteAuth should not error: %v", err)
	}
}

// ── Stats ─────────────────────────────────────────────────────────────────────

func TestIncrementStats(t *testing.T) {
	s := newStore(t)
	if err := s.IncrementStats("op-1", 100, false); err != nil {
		t.Fatalf("IncrementStats: %v", err)
	}
	if err := s.IncrementStats("op-1", 200, true); err != nil {
		t.Fatal(err)
	}
	st, err := s.GetStats("op-1")
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if st.CallCount != 2 {
		t.Errorf("CallCount = %d, want 2", st.CallCount)
	}
	if st.ErrorCount != 1 {
		t.Errorf("ErrorCount = %d, want 1", st.ErrorCount)
	}
	if st.TotalLatencyMs != 300 {
		t.Errorf("TotalLatencyMs = %d, want 300", st.TotalLatencyMs)
	}
}

func TestGetStatsNotFound(t *testing.T) {
	s := newStore(t)
	_, err := s.GetStats("no-such-op")
	if err == nil {
		t.Error("expected error for nonexistent stats")
	}
}

func TestGetAllStats(t *testing.T) {
	s := newStore(t)
	s.IncrementStats("op-a", 10, false) //nolint:errcheck
	s.IncrementStats("op-b", 20, false) //nolint:errcheck

	all, err := s.GetAllStats()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Errorf("GetAllStats() = %d entries, want 2", len(all))
	}
}

// ── Resources ─────────────────────────────────────────────────────────────────

func TestSaveAndGetResource(t *testing.T) {
	s := newStore(t)
	rec := &store.ResourceRecord{
		ID:       "res-1",
		Name:     "My Resource",
		Type:     "text",
		MimeType: "text/plain",
	}
	if err := s.SaveResource(rec); err != nil {
		t.Fatalf("SaveResource: %v", err)
	}
	got, err := s.GetResource("res-1")
	if err != nil {
		t.Fatalf("GetResource: %v", err)
	}
	if got.Name != "My Resource" {
		t.Errorf("Name = %q", got.Name)
	}
}

func TestListResources(t *testing.T) {
	s := newStore(t)
	for _, id := range []string{"r1", "r2"} {
		s.SaveResource(&store.ResourceRecord{ID: id, Name: id}) //nolint:errcheck
	}
	resources, err := s.ListResources()
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 2 {
		t.Errorf("ListResources() = %d, want 2", len(resources))
	}
}

func TestDeleteResource(t *testing.T) {
	s := newStore(t)
	s.SaveResource(&store.ResourceRecord{ID: "del-res"}) //nolint:errcheck
	if err := s.DeleteResource("del-res"); err != nil {
		t.Fatalf("DeleteResource: %v", err)
	}
	_, err := s.GetResource("del-res")
	if err == nil {
		t.Error("resource should have been deleted")
	}
}

func TestDeleteResourceNotFound(t *testing.T) {
	s := newStore(t)
	err := s.DeleteResource("nonexistent")
	if err == nil {
		t.Error("expected error deleting nonexistent resource")
	}
}

func TestSafeJoin(t *testing.T) {
base := t.TempDir()

// Normal subpath
p, err := store.SafeJoin(base, "resources/abc/file.txt")
if err != nil {
	t.Fatalf("unexpected error: %v", err)
}
if p == "" {
	t.Error("expected non-empty path")
}

// Traversal attempt via ".."
_, err = store.SafeJoin(base, "../outside/file.txt")
if err == nil {
	t.Error("expected error for path traversal via ..")
}

// Note: Go's filepath.Join handles leading "/" by joining it under base,
// so "/etc/passwd" becomes base+"/etc/passwd" — not an escape vector in Go.
// The SafeJoin implementation correctly confines such paths within base.
absSafe, err := store.SafeJoin(base, "/etc/passwd")
if err != nil {
	t.Fatalf("unexpected error for absolute path (Go filepath.Join keeps it in base): %v", err)
}
if !strings.HasPrefix(absSafe, base) {
	t.Errorf("absolute path escaped base: %s not under %s", absSafe, base)
}

// Empty path
_, err = store.SafeJoin(base, "")
if err == nil {
	t.Error("expected error for empty path")
}
}
