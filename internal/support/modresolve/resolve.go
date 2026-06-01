// Package modresolve contains shared require() path resolution rules.
package modresolve

import (
	"path/filepath"
	"strings"
)

type Collection struct {
	Name string
	Root string
}

type Replace struct {
	Path string
	Root string
}

type Result struct {
	Kind string
	Path string
	Root string
	Rel  string
	File string
}

func Resolve(module string, collections []Collection, replaces []Replace, root string) Result {
	if result, ok := ResolveCollection(module, collections); ok {
		return result
	}
	if result, ok := ResolveReplace(module, replaces); ok {
		return result
	}
	file := moduleFile(module)
	if root != "" {
		file = filepath.Join(root, file)
	}
	return Result{Kind: "module", Root: root, Rel: moduleFile(module), File: file}
}

func ResolveCollection(module string, collections []Collection) (Result, bool) {
	idx := strings.Index(module, ":")
	if idx <= 0 {
		return Result{}, false
	}
	prefix := module[:idx]
	for _, col := range collections {
		if col.Name != prefix || col.Root == "" {
			continue
		}
		rel := strings.ReplaceAll(module[idx+1:], ".", "/") + ".gs"
		return Result{Kind: "collection", Path: col.Name, Root: col.Root, Rel: rel, File: filepath.Join(col.Root, rel)}, true
	}
	return Result{}, false
}

func ResolveReplace(module string, replaces []Replace) (Result, bool) {
	var best Replace
	for _, rep := range replaces {
		if rep.Path == "" || rep.Root == "" {
			continue
		}
		if module != rep.Path && !strings.HasPrefix(module, rep.Path+"/") {
			continue
		}
		if len(rep.Path) > len(best.Path) {
			best = rep
		}
	}
	if best.Path == "" {
		return Result{}, false
	}
	if module == best.Path {
		if filepath.Ext(best.Root) == ".gs" {
			return Result{Kind: "replace", Path: best.Path, Root: filepath.Dir(best.Root), Rel: filepath.Base(best.Root), File: best.Root}, true
		}
		file := best.Root + ".gs"
		return Result{Kind: "replace", Path: best.Path, Root: filepath.Dir(file), Rel: filepath.Base(file), File: file}, true
	}
	rel := strings.TrimPrefix(module[len(best.Path):], "/")
	rel = strings.ReplaceAll(rel, ".", "/") + ".gs"
	return Result{Kind: "replace", Path: best.Path, Root: best.Root, Rel: rel, File: filepath.Join(best.Root, rel)}, true
}

func moduleFile(module string) string {
	if strings.Contains(module, "/") || strings.HasPrefix(module, ".") {
		return module + ".gs"
	}
	return strings.ReplaceAll(module, ".", "/") + ".gs"
}
