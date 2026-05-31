package fs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveSandboxPath resolves path under root and rejects escapes. Empty root
// preserves the host path unchanged for unrestricted embedders.
func ResolveSandboxPath(root, path string) (string, error) {
	if root == "" {
		return path, nil
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	absRoot = filepath.Clean(absRoot)
	var candidate string
	if filepath.IsAbs(path) {
		candidate = filepath.Clean(path)
	} else {
		candidate = filepath.Clean(filepath.Join(absRoot, path))
	}
	absCandidate, err := filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	absCandidate = filepath.Clean(absCandidate)
	if absCandidate == absRoot {
		return absCandidate, nil
	}
	prefix := absRoot + string(os.PathSeparator)
	if strings.HasPrefix(absCandidate, prefix) {
		return absCandidate, nil
	}
	return "", fmt.Errorf("filesystem access denied: path escapes root")
}
