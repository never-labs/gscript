package gscript

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/never-labs/gscript/internal/modfile"
	"github.com/never-labs/gscript/internal/modpkg"
	"github.com/never-labs/gscript/internal/support/modresolve"
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
	for _, rep := range manifest.Replace {
		if !isLocalModulePath(rep.NewPath) {
			continue
		}
		root := rep.NewPath
		if !filepath.IsAbs(root) {
			root = filepath.Join(dir, root)
		}
		opts = append(opts, WithModuleReplace(rep.Path, root))
	}
	if modules := vendorModulesForManifest(dir, manifest); len(modules) > 0 {
		opts = append(opts, withModuleCacheModules(modules))
	}
	if cacheDir, err := modpkg.ModuleCacheDir(""); err == nil {
		if modules := cacheModulesForManifest(cacheDir, manifest); len(modules) > 0 {
			opts = append(opts, WithModuleCache(cacheDir), withModuleCacheModules(modules))
		}
	}
	return opts
}

func vendorModulesForManifest(root string, manifest modfile.File) []modresolve.CacheModule {
	modules := make([]modresolve.CacheModule, 0, len(manifest.Require))
	for _, req := range manifest.Require {
		if req.Version == "" {
			continue
		}
		vendorRoot := filepath.Join(root, "vendor", filepath.FromSlash(req.Path+"@"+req.Version))
		if _, err := os.Stat(vendorRoot); err != nil {
			continue
		}
		modules = append(modules, modresolve.CacheModule{Path: req.Path, Version: req.Version, Root: vendorRoot})
	}
	return modules
}

func cacheModulesForManifest(cacheDir string, manifest modfile.File) []modresolve.CacheModule {
	modules := make([]modresolve.CacheModule, 0, len(manifest.Require))
	for _, req := range manifest.Require {
		if !strings.HasPrefix(req.Path, "github.com/") || req.Version == "" {
			continue
		}
		root := filepath.Join(cacheDir, "extract", filepath.FromSlash(req.Path+"@"+req.Version))
		if _, err := os.Stat(root); err != nil {
			// A missing cache entry is not an error here. Runtime loading remains
			// offline and will fail normally if the file is unavailable.
			continue
		}
		modules = append(modules, modresolve.CacheModule{Path: req.Path, Version: req.Version, Root: root})
	}
	return modules
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

func isLocalModulePath(path string) bool {
	return strings.HasPrefix(path, ".") || filepath.IsAbs(path)
}
