package store

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SafeJoin joins base and untrusted to form a path and verifies the result
// is strictly within base. It returns an error if the resolved path escapes
// base via ".." traversal or absolute path injection.
func SafeJoin(base, untrusted string) (string, error) {
	if untrusted == "" {
		return "", fmt.Errorf("empty path")
	}
	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", fmt.Errorf("resolving base path: %w", err)
	}
	joined := filepath.Join(absBase, untrusted)
	// Ensure the joined path is inside base (append separator to prevent
	// prefix-match false positives like /data/foobar matching /data/foo).
	if !strings.HasPrefix(joined, absBase+string(filepath.Separator)) && joined != absBase {
		return "", fmt.Errorf("path %q escapes data directory", untrusted)
	}
	return joined, nil
}
