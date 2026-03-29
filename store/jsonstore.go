package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type JSONStore struct {
	dataDir    string
	statsMu    sync.Mutex
	statsCache map[string]*ToolStats
	statsDirty bool
	stopCh     chan struct{}
	doneCh     chan struct{}
}

func NewJSONStore(dataDir string) (*JSONStore, error) {
	dirs := []string{
		filepath.Join(dataDir, "specs"),
		filepath.Join(dataDir, "operations"),
		filepath.Join(dataDir, "auth"),
		filepath.Join(dataDir, "stats"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return nil, err
		}
	}

	s := &JSONStore{
		dataDir:    dataDir,
		statsCache: make(map[string]*ToolStats),
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
	}

	statsFile := filepath.Join(dataDir, "stats", "tool_stats.json")
	if data, err := os.ReadFile(statsFile); err == nil {
		if err := json.Unmarshal(data, &s.statsCache); err != nil {
			s.statsCache = make(map[string]*ToolStats)
		}
	}

	go s.flushLoop()
	return s, nil
}

func (s *JSONStore) flushLoop() {
	defer close(s.doneCh)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.flushStats()
		case <-s.stopCh:
			s.flushStats()
			return
		}
	}
}

func (s *JSONStore) flushStats() {
	s.statsMu.Lock()
	if !s.statsDirty {
		s.statsMu.Unlock()
		return
	}
	copyMap := make(map[string]*ToolStats, len(s.statsCache))
	for k, v := range s.statsCache {
		cp := *v
		copyMap[k] = &cp
	}
	s.statsDirty = false
	s.statsMu.Unlock()

	statsFile := filepath.Join(s.dataDir, "stats", "tool_stats.json")
	_ = writeJSONAtomic(statsFile, copyMap)
}

func (s *JSONStore) Close() error {
	close(s.stopCh)
	<-s.doneCh
	return nil
}

func writeJSONAtomic(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readJSON(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func (s *JSONStore) SaveSpec(spec *SpecRecord) error {
	path := filepath.Join(s.dataDir, "specs", spec.ID+".json")
	return writeJSONAtomic(path, spec)
}

func (s *JSONStore) GetSpec(id string) (*SpecRecord, error) {
	var rec SpecRecord
	path := filepath.Join(s.dataDir, "specs", id+".json")
	if err := readJSON(path, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

func (s *JSONStore) ListSpecs() ([]*SpecRecord, error) {
	dir := filepath.Join(s.dataDir, "specs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var specs []*SpecRecord
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		id := e.Name()[:len(e.Name())-5]
		rec, err := s.GetSpec(id)
		if err != nil {
			continue
		}
		specs = append(specs, rec)
	}
	return specs, nil
}

func (s *JSONStore) DeleteSpec(id string) error {
	path := filepath.Join(s.dataDir, "specs", id+".json")
	return os.Remove(path)
}

func (s *JSONStore) SaveOperations(specID string, ops []*OperationRecord) error {
	path := filepath.Join(s.dataDir, "operations", specID+".json")
	return writeJSONAtomic(path, ops)
}

func (s *JSONStore) GetOperations(specID string) ([]*OperationRecord, error) {
	var ops []*OperationRecord
	path := filepath.Join(s.dataDir, "operations", specID+".json")
	if err := readJSON(path, &ops); err != nil {
		return nil, err
	}
	return ops, nil
}

func (s *JSONStore) UpdateOperation(specID string, op *OperationRecord) error {
	ops, err := s.GetOperations(specID)
	if err != nil {
		return err
	}
	for i, o := range ops {
		if o.ID == op.ID {
			ops[i] = op
			return s.SaveOperations(specID, ops)
		}
	}
	return fmt.Errorf("operation %s not found in spec %s", op.ID, specID)
}

func (s *JSONStore) DeleteOperations(specID string) error {
	path := filepath.Join(s.dataDir, "operations", specID+".json")
	return os.Remove(path)
}

func (s *JSONStore) SaveAuth(specID string, cfg *AuthConfig) error {
	path := filepath.Join(s.dataDir, "auth", specID+".json")
	return writeJSONAtomic(path, cfg)
}

func (s *JSONStore) GetAuth(specID string) (*AuthConfig, error) {
	var cfg AuthConfig
	path := filepath.Join(s.dataDir, "auth", specID+".json")
	if err := readJSON(path, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (s *JSONStore) DeleteAuth(specID string) error {
	path := filepath.Join(s.dataDir, "auth", specID+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *JSONStore) IncrementStats(operationID string, latencyMs int64, isError bool) error {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()

	st, ok := s.statsCache[operationID]
	if !ok {
		st = &ToolStats{OperationID: operationID}
		s.statsCache[operationID] = st
	}
	st.CallCount++
	if isError {
		st.ErrorCount++
	}
	st.TotalLatencyMs += latencyMs
	st.LastCalledAt = time.Now()
	s.statsDirty = true
	return nil
}

func (s *JSONStore) GetAllStats() (map[string]*ToolStats, error) {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()

	copyMap := make(map[string]*ToolStats, len(s.statsCache))
	for k, v := range s.statsCache {
		cp := *v
		copyMap[k] = &cp
	}
	return copyMap, nil
}

func (s *JSONStore) GetStats(operationID string) (*ToolStats, error) {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()

	st, ok := s.statsCache[operationID]
	if !ok {
		return nil, fmt.Errorf("stats not found for operation %s", operationID)
	}
	cp := *st
	return &cp, nil
}
