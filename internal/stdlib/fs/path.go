package fs

import "github.com/never-labs/leia/internal/support/hostpath"

// ResolveSandboxPath resolves path under root and rejects escapes. Empty root
// preserves the host path unchanged for unrestricted embedders.
func ResolveSandboxPath(root, path string) (string, error) {
	return hostpath.ResolveSandboxPath(root, path)
}
