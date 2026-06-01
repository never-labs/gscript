package gscript

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/never-labs/gscript/internal/modfile"
)

// ModuleOptionsForScript discovers the nearest gscript.mod for script and
// returns module-loading options derived from it. It intentionally does not
// fetch packages or mutate files.
func ModuleOptionsForScript(script string) []Option {
	start := "."
	if script != "" {
		if abs, err := filepath.Abs(script); err == nil {
			start = filepath.Dir(abs)
		} else {
			start = filepath.Dir(script)
		}
	}
	dir, file, ok := findModuleFile(start)
	if !ok {
		return nil
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	manifest, diags := modfile.Parse(file, strings.NewReader(string(data)))
	if len(diags) > 0 {
		return nil
	}
	opts := []Option{WithRequirePath(dir)}
	for _, col := range manifest.Collections {
		root := col.Path
		if !filepath.IsAbs(root) {
			root = filepath.Join(dir, root)
		}
		opts = append(opts, WithModuleCollection(col.Name, root))
	}
	return opts
}

func findModuleFile(start string) (string, string, bool) {
	dir, err := filepath.Abs(start)
	if err != nil {
		dir = start
	}
	for {
		candidate := filepath.Join(dir, modfile.FileName)
		if _, err := os.Stat(candidate); err == nil {
			return dir, candidate, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "", false
		}
		dir = parent
	}
}
