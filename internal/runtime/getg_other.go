//go:build !arm64

package runtime

// getgptr returns 0 on platforms without an assembly implementation.
// Value scopes are inert there: PushValueScope returns nil and every
// root-log append goes to the global log, which is always safe.
func getgptr() uintptr { return 0 }
