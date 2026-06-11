//go:build arm64

package runtime

// getgptr returns the current goroutine's g pointer as an opaque identity
// token. It is implemented in assembly (getg_arm64.s). The token is only
// compared for equality; it is never dereferenced and never stored as a
// GC-visible pointer.
func getgptr() uintptr
