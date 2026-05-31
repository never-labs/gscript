package fs

import "os"

// DirEntry is the script-facing projection of an os.DirEntry.
type DirEntry struct {
	Name  string
	IsDir bool
	Size  int64
}

// ProjectDirEntry converts an os.DirEntry into the stable shape exposed by
// fs.readdir. Size falls back to zero when stat metadata is unavailable.
func ProjectDirEntry(entry os.DirEntry) DirEntry {
	size := int64(0)
	if info, err := entry.Info(); err == nil {
		size = info.Size()
	}
	return DirEntry{
		Name:  entry.Name(),
		IsDir: entry.IsDir(),
		Size:  size,
	}
}

// ProjectDirEntries converts entries in order without sorting or filtering.
func ProjectDirEntries(entries []os.DirEntry) []DirEntry {
	projected := make([]DirEntry, len(entries))
	for i, entry := range entries {
		projected[i] = ProjectDirEntry(entry)
	}
	return projected
}
