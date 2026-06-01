// Package modfile parses and formats Leia module manifests.
package modfile

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode"
)

const FileName = "leia.mod"

type File struct {
	Module      string
	Leia          string
	Go          string
	Capability  []string
	Require     []Require
	GoRequire   []Require
	Replace     []Replace
	GoReplace   []Replace
	Collections []Collection
}

type Require struct {
	Path    string
	Version string
}

type Replace struct {
	Path    string
	Version string
	NewPath string
}

type Collection struct {
	Name string
	Path string
}

type Diagnostic struct {
	Line    int
	Message string
}

func AddRequire(f File, req Require) (File, error) {
	if !validModulePath(req.Path) {
		return f, fmt.Errorf("invalid require path")
	}
	if strings.TrimSpace(req.Version) == "" {
		return f, fmt.Errorf("require version is required")
	}
	for i := range f.Require {
		if f.Require[i].Path == req.Path {
			f.Require[i].Version = req.Version
			return f, nil
		}
	}
	f.Require = append(f.Require, req)
	return f, nil
}

func DropRequire(f File, path string) File {
	out := f.Require[:0]
	for _, req := range f.Require {
		if req.Path != path {
			out = append(out, req)
		}
	}
	f.Require = out
	return f
}

func Parse(name string, r io.Reader) (File, []Diagnostic) {
	var f File
	var diags []Diagnostic
	seenRequire := map[string]int{}
	seenReplace := map[string]int{}
	seenGoRequire := map[string]int{}
	seenGoReplace := map[string]int{}
	seenCollection := map[string]bool{}
	seenCapability := map[string]bool{}
	seenGS := 0
	seenGo := 0

	scanner := bufio.NewScanner(r)
	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := stripComment(scanner.Text())
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "module":
			if len(fields) != 2 {
				diags = append(diags, diag(lineNo, "module expects one path"))
				continue
			}
			if f.Module != "" {
				diags = append(diags, diag(lineNo, "module declared more than once"))
				continue
			}
			if !validModulePath(fields[1]) {
				diags = append(diags, diag(lineNo, "invalid module path"))
				continue
			}
			f.Module = fields[1]
		case "leia":
			if len(fields) != 2 {
				diags = append(diags, diag(lineNo, "leia expects one language version"))
				continue
			}
			if seenGS != 0 {
				diags = append(diags, diag(lineNo, fmt.Sprintf("leia declared more than once; first declared on line %d", seenGS)))
				continue
			}
			seenGS = lineNo
			f.Leia = fields[1]
		case "go":
			switch {
			case len(fields) == 2:
				if seenGo != 0 {
					diags = append(diags, diag(lineNo, fmt.Sprintf("go declared more than once; first declared on line %d", seenGo)))
					continue
				}
				seenGo = lineNo
				f.Go = fields[1]
			case len(fields) == 4 && fields[1] == "require":
				if !validModulePath(fields[2]) {
					diags = append(diags, diag(lineNo, "invalid go require path"))
					continue
				}
				if prevLine, ok := seenGoRequire[fields[2]]; ok {
					diags = append(diags, diag(lineNo, fmt.Sprintf("duplicate go require for %s; first declared on line %d", fields[2], prevLine)))
					continue
				}
				seenGoRequire[fields[2]] = lineNo
				f.GoRequire = append(f.GoRequire, Require{Path: fields[2], Version: fields[3]})
			case len(fields) >= 5 && fields[1] == "replace":
				idx := indexField(fields, "=>")
				if idx == -1 || idx < 3 || idx >= len(fields)-1 {
					diags = append(diags, diag(lineNo, "go replace expects: go replace path [version] => path"))
					continue
				}
				old := fields[2:idx]
				if len(old) > 2 {
					diags = append(diags, diag(lineNo, "go replace old side expects path and optional version"))
					continue
				}
				if !validModulePath(old[0]) {
					diags = append(diags, diag(lineNo, "invalid go replace path"))
					continue
				}
				if len(fields[idx+1:]) != 1 {
					diags = append(diags, diag(lineNo, "go replace new side expects one path"))
					continue
				}
				rep := Replace{Path: old[0], NewPath: fields[idx+1]}
				if len(old) == 2 {
					rep.Version = old[1]
				}
				key := rep.Path + "\x00" + rep.Version
				if prevLine, ok := seenGoReplace[key]; ok {
					diags = append(diags, diag(lineNo, fmt.Sprintf("duplicate go replace for %s; first declared on line %d", rep.Path, prevLine)))
					continue
				}
				seenGoReplace[key] = lineNo
				f.GoReplace = append(f.GoReplace, rep)
			default:
				diags = append(diags, diag(lineNo, "go expects version, require, or replace"))
			}
		case "require":
			if len(fields) != 3 {
				diags = append(diags, diag(lineNo, "require expects path and version"))
				continue
			}
			if !validModulePath(fields[1]) {
				diags = append(diags, diag(lineNo, "invalid require path"))
				continue
			}
			if prevLine, ok := seenRequire[fields[1]]; ok {
				diags = append(diags, diag(lineNo, fmt.Sprintf("duplicate require for %s; first declared on line %d", fields[1], prevLine)))
				continue
			}
			seenRequire[fields[1]] = lineNo
			f.Require = append(f.Require, Require{Path: fields[1], Version: fields[2]})
		case "cap", "capability":
			caps, err := parseCapabilityFields(fields[1:])
			if err != nil {
				diags = append(diags, diag(lineNo, err.Error()))
				continue
			}
			for _, cap := range caps {
				if seenCapability[cap] {
					continue
				}
				seenCapability[cap] = true
				f.Capability = append(f.Capability, cap)
			}
		case "replace":
			idx := indexField(fields, "=>")
			if idx == -1 || idx < 2 || idx >= len(fields)-1 {
				diags = append(diags, diag(lineNo, "replace expects: replace path [version] => path"))
				continue
			}
			old := fields[1:idx]
			if len(old) > 2 {
				diags = append(diags, diag(lineNo, "replace old side expects path and optional version"))
				continue
			}
			if !validModulePath(old[0]) {
				diags = append(diags, diag(lineNo, "invalid replace path"))
				continue
			}
			if len(fields[idx+1:]) != 1 {
				diags = append(diags, diag(lineNo, "replace new side expects one path"))
				continue
			}
			rep := Replace{Path: old[0], NewPath: fields[idx+1]}
			if len(old) == 2 {
				rep.Version = old[1]
			}
			key := rep.Path + "\x00" + rep.Version
			if prevLine, ok := seenReplace[key]; ok {
				diags = append(diags, diag(lineNo, fmt.Sprintf("duplicate replace for %s; first declared on line %d", rep.Path, prevLine)))
				continue
			}
			seenReplace[key] = lineNo
			f.Replace = append(f.Replace, rep)
		case "collection":
			if len(fields) != 3 {
				diags = append(diags, diag(lineNo, "collection expects name and path"))
				continue
			}
			if !validCollectionName(fields[1]) {
				diags = append(diags, diag(lineNo, "invalid collection name"))
				continue
			}
			if seenCollection[fields[1]] {
				diags = append(diags, diag(lineNo, "duplicate collection"))
				continue
			}
			seenCollection[fields[1]] = true
			f.Collections = append(f.Collections, Collection{Name: fields[1], Path: fields[2]})
		default:
			diags = append(diags, diag(lineNo, fmt.Sprintf("unknown directive %q", fields[0])))
		}
	}
	if err := scanner.Err(); err != nil {
		diags = append(diags, Diagnostic{Message: name + ": " + err.Error()})
	}
	if f.Module == "" {
		diags = append(diags, Diagnostic{Message: "module is required"})
	}
	if f.Leia == "" {
		f.Leia = "0.1"
	}
	return f, diags
}

func Format(f File) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "module %s\n", f.Module)
	if f.Leia != "" {
		fmt.Fprintf(&b, "leia %s\n", f.Leia)
	}
	writeGo(&b, f.Go, f.GoRequire, f.GoReplace)
	writeCapabilities(&b, f.Capability)
	writeRequires(&b, f.Require)
	writeReplaces(&b, f.Replace)
	writeCollections(&b, f.Collections)
	return []byte(b.String())
}

func writeGo(b *strings.Builder, version string, reqs []Require, reps []Replace) {
	if version == "" && len(reqs) == 0 && len(reps) == 0 {
		return
	}
	b.WriteByte('\n')
	if version != "" {
		fmt.Fprintf(b, "go %s\n", version)
	}
	sort.Slice(reqs, func(i, j int) bool { return reqs[i].Path < reqs[j].Path })
	for _, req := range reqs {
		fmt.Fprintf(b, "go require %s %s\n", req.Path, req.Version)
	}
	sort.Slice(reps, func(i, j int) bool { return reps[i].Path < reps[j].Path })
	for _, rep := range reps {
		if rep.Version != "" {
			fmt.Fprintf(b, "go replace %s %s => %s\n", rep.Path, rep.Version, rep.NewPath)
		} else {
			fmt.Fprintf(b, "go replace %s => %s\n", rep.Path, rep.NewPath)
		}
	}
}

func stripComment(line string) string {
	if idx := strings.Index(line, "//"); idx >= 0 {
		line = line[:idx]
	}
	return strings.TrimSpace(line)
}

func diag(line int, msg string) Diagnostic {
	return Diagnostic{Line: line, Message: msg}
}

func indexField(fields []string, value string) int {
	for i, field := range fields {
		if field == value {
			return i
		}
	}
	return -1
}

func validModulePath(path string) bool {
	if path == "" || strings.Contains(path, "\\") || strings.Contains(path, "..") {
		return false
	}
	for _, r := range path {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		switch r {
		case '.', '-', '_', '/', ':':
			continue
		default:
			return false
		}
	}
	return true
}

func validCollectionName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func parseCapabilityFields(fields []string) ([]string, error) {
	if len(fields) == 0 {
		return nil, fmt.Errorf("capability expects at least one name")
	}
	seen := map[string]bool{}
	var caps []string
	for _, field := range fields {
		for _, part := range strings.Split(field, ",") {
			cap := strings.TrimSpace(part)
			if cap == "" {
				continue
			}
			if !validCapabilityName(cap) {
				return nil, fmt.Errorf("invalid capability name")
			}
			if seen[cap] {
				continue
			}
			seen[cap] = true
			caps = append(caps, cap)
		}
	}
	if len(caps) == 0 {
		return nil, fmt.Errorf("invalid capability name")
	}
	return caps, nil
}

func validCapabilityName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		switch r {
		case '.', '-', '_', '/', ':':
			continue
		default:
			return false
		}
	}
	return true
}

func writeCapabilities(b *strings.Builder, caps []string) {
	if len(caps) == 0 {
		return
	}
	sort.Strings(caps)
	b.WriteByte('\n')
	for _, cap := range caps {
		fmt.Fprintf(b, "capability %s\n", cap)
	}
}

func writeRequires(b *strings.Builder, reqs []Require) {
	if len(reqs) == 0 {
		return
	}
	sort.Slice(reqs, func(i, j int) bool { return reqs[i].Path < reqs[j].Path })
	b.WriteByte('\n')
	for _, req := range reqs {
		fmt.Fprintf(b, "require %s %s\n", req.Path, req.Version)
	}
}

func writeReplaces(b *strings.Builder, reps []Replace) {
	if len(reps) == 0 {
		return
	}
	sort.Slice(reps, func(i, j int) bool { return reps[i].Path < reps[j].Path })
	b.WriteByte('\n')
	for _, rep := range reps {
		if rep.Version != "" {
			fmt.Fprintf(b, "replace %s %s => %s\n", rep.Path, rep.Version, rep.NewPath)
		} else {
			fmt.Fprintf(b, "replace %s => %s\n", rep.Path, rep.NewPath)
		}
	}
}

func writeCollections(b *strings.Builder, cols []Collection) {
	if len(cols) == 0 {
		return
	}
	sort.Slice(cols, func(i, j int) bool { return cols[i].Name < cols[j].Name })
	b.WriteByte('\n')
	for _, col := range cols {
		fmt.Fprintf(b, "collection %s %s\n", col.Name, col.Path)
	}
}
