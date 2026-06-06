package bind

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func expandHostGlobSpec(root, spec string) ([]string, error) {
	includes, excludes := splitGlobSpec(spec)
	if len(includes) == 0 && len(excludes) == 0 {
		includes = []string{spec}
	}

	all := map[string]struct{}{}
	for _, pattern := range includes {
		matches, err := expandHostGlobPattern(root, pattern)
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			all[match] = struct{}{}
		}
	}

	for _, pattern := range excludes {
		matches, err := expandHostGlobPattern(root, pattern)
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			delete(all, match)
		}
	}

	out := make([]string, 0, len(all))
	for match := range all {
		out = append(out, match)
	}
	sort.Strings(out)
	return out, nil
}

func splitGlobSpec(spec string) (includes, excludes []string) {
	lines := strings.Split(spec, "\n")
	if len(lines) == 1 {
		trimmed := strings.TrimSpace(spec)
		if strings.HasPrefix(trimmed, "!") {
			return nil, []string{strings.TrimSpace(strings.TrimPrefix(trimmed, "!"))}
		}
		return []string{spec}, nil
	}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "!") {
			pattern := strings.TrimSpace(strings.TrimPrefix(line, "!"))
			if pattern != "" {
				excludes = append(excludes, pattern)
			}
			continue
		}
		includes = append(includes, line)
	}
	return includes, excludes
}

func expandHostGlobPattern(root, pattern string) ([]string, error) {
	resolved, err := resolveSandboxPath(root, pattern)
	if err != nil {
		return nil, err
	}
	if !strings.Contains(resolved, "**") {
		return filepath.Glob(resolved)
	}
	if err := validateDoubleStarGlob(resolved); err != nil {
		return nil, err
	}
	walkRoot := globWalkRoot(resolved)
	matches := make([]string, 0)
	err = filepath.WalkDir(walkRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		matched, err := doubleStarMatch(resolved, path)
		if err != nil {
			return err
		}
		if matched {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		if filepath.IsAbs(walkRoot) || walkRoot == "." {
			return nil, err
		}
		return []string{}, nil
	}
	sort.Strings(matches)
	return matches, nil
}

func validateDoubleStarGlob(pattern string) error {
	for _, segment := range splitPathSegments(pattern) {
		if segment == "**" {
			continue
		}
		if _, err := filepath.Match(segment, ""); err != nil {
			return err
		}
	}
	return nil
}

func globWalkRoot(pattern string) string {
	volume := filepath.VolumeName(pattern)
	rest := strings.TrimPrefix(pattern, volume)
	sep := string(filepath.Separator)
	absolute := strings.HasPrefix(rest, sep)
	rest = strings.TrimPrefix(rest, sep)
	parts := strings.Split(rest, sep)

	prefix := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		if part == "**" || strings.ContainsAny(part, "*?[") {
			break
		}
		prefix = append(prefix, part)
	}

	if volume != "" || absolute {
		base := volume + sep
		if len(prefix) == 0 {
			return filepath.Clean(base)
		}
		all := append([]string{base}, prefix...)
		return filepath.Join(all...)
	}
	if len(prefix) == 0 {
		return "."
	}
	return filepath.Join(prefix...)
}

func doubleStarMatch(pattern, name string) (bool, error) {
	patternParts := splitPathSegments(pattern)
	nameParts := splitPathSegments(name)
	var match func(int, int) (bool, error)
	match = func(pi, ni int) (bool, error) {
		if pi == len(patternParts) {
			return ni == len(nameParts), nil
		}
		if patternParts[pi] == "**" {
			if ok, err := match(pi+1, ni); ok || err != nil {
				return ok, err
			}
			if ni < len(nameParts) {
				return match(pi, ni+1)
			}
			return false, nil
		}
		if ni >= len(nameParts) {
			return false, nil
		}
		ok, err := filepath.Match(patternParts[pi], nameParts[ni])
		if !ok || err != nil {
			return ok, err
		}
		return match(pi+1, ni+1)
	}
	return match(0, 0)
}

func splitPathSegments(path string) []string {
	slash := filepath.ToSlash(filepath.Clean(path))
	parts := strings.Split(slash, "/")
	out := parts[:0]
	for i, part := range parts {
		if part == "" && i > 0 {
			continue
		}
		out = append(out, part)
	}
	return out
}
