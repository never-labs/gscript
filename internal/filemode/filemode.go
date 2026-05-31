package filemode

import (
	"os"
	"strings"
)

// Parse converts the script-facing io.open mode string into os.OpenFile flags.
func Parse(mode string) (int, bool) {
	mode = strings.ReplaceAll(mode, "b", "")
	switch mode {
	case "r":
		return os.O_RDONLY, true
	case "w":
		return os.O_WRONLY | os.O_CREATE | os.O_TRUNC, true
	case "a":
		return os.O_WRONLY | os.O_CREATE | os.O_APPEND, true
	case "r+":
		return os.O_RDWR, true
	case "w+":
		return os.O_RDWR | os.O_CREATE | os.O_TRUNC, true
	case "a+":
		return os.O_RDWR | os.O_CREATE | os.O_APPEND, true
	default:
		return 0, false
	}
}

// Access reports which filesystem capabilities are required by flags.
func Access(flag int) (read, write bool) {
	read = flag&os.O_WRONLY == 0 || flag&os.O_RDWR != 0
	write = flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND) != 0
	return read, write
}
