package fs

import (
	"fmt"
	"os"
)

// FileInfo is the script-facing projection of os.FileInfo used by fs.stat.
type FileInfo struct {
	Name  string
	Size  int64
	MTime int64
	IsDir bool
	Mode  string
}

// IsFile reports whether the projected entry should be exposed as a file.
func (info FileInfo) IsFile() bool {
	return !info.IsDir
}

// FormatPerm formats permission bits in the legacy fs.stat shape.
func FormatPerm(mode os.FileMode) string {
	return fmt.Sprintf("0%o", mode.Perm())
}

// ProjectFileInfo converts os.FileInfo into the stable shape exposed by
// fs.stat without performing filesystem operations.
func ProjectFileInfo(info os.FileInfo) FileInfo {
	return FileInfo{
		Name:  info.Name(),
		Size:  info.Size(),
		MTime: info.ModTime().Unix(),
		IsDir: info.IsDir(),
		Mode:  FormatPerm(info.Mode()),
	}
}
