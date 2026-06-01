package llm

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// SnapshotToken generates the opaque token used by human-in-the-loop agent
// snapshots. It is independent from runtime Value/Table adapters.
func SnapshotToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("loop.snapshot: failed to generate token: %v", err)
	}
	return hex.EncodeToString(b[:]), nil
}
