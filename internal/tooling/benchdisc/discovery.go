package benchdisc

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

var DefaultOrder = []string{
	"fib",
	"fib_recursive",
	"sieve",
	"mandelbrot",
	"ackermann",
	"matmul",
	"spectral_norm",
	"nbody",
	"fannkuch",
	"sort",
	"sum_primes",
	"mutual_recursion",
	"method_dispatch",
	"closure_bench",
	"string_bench",
	"binary_trees",
	"table_field_access",
	"table_array_access",
	"coroutine_bench",
	"fibonacci_iterative",
	"math_intensive",
	"object_creation",
}

var DomainGroups = []string{"numeric", "recursion", "table", "calls", "string", "concurrency", "data", "app", "control", "precision"}

var RelatedBenchmarkBases = map[string]string{
	"ack_nested_shifted":  "ackermann",
	"sort_mixed_numeric":  "sort",
	"matmul_row":          "matmul",
	"closure_accumulator": "closure_bench",
}

type Benchmark struct {
	Group  string
	Name   string
	Leia   string
	LuaJIT string
	Base   string
}

func (b Benchmark) ID() string {
	return b.Group + "/" + b.Name
}

func (b Benchmark) LeiaRel() string {
	return filepath.ToSlash(filepath.Join("benchmarks", b.Group, b.Name+".leia"))
}

func (b Benchmark) LuaJITRel() string {
	if b.LuaJIT == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Join("benchmarks", "lua_ref", b.Group, b.Name+".lua"))
}

type Selectable interface {
	ID() string
}

func GroupChoices(allowed []string) []string {
	return append([]string(nil), allowed...)
}

func BenchmarkIDFromSelector(selector string, allowedGroups []string) (string, bool) {
	text := strings.TrimPrefix(selector, "benchmarks/")
	text = strings.TrimSuffix(text, ".leia")
	parts := strings.Split(text, "/")
	if len(parts) != 2 || parts[1] == "" {
		return "", false
	}
	if !contains(allowedGroups, parts[0]) {
		return "", false
	}
	return parts[0] + "/" + parts[1], true
}

func SelectorMatches(selector string, allowed map[string]struct{}) bool {
	if id, ok := BenchmarkIDFromSelector(selector, DomainGroups); ok {
		_, exists := allowed[id]
		return exists
	}
	_, exists := allowed[selector]
	return exists
}

func SpecSelectors[T Selectable](specs []T) map[string]struct{} {
	out := make(map[string]struct{}, len(specs))
	for _, spec := range specs {
		out[spec.ID()] = struct{}{}
	}
	return out
}

func SelectorMatchesSpec[T Selectable](selector string, spec T) bool {
	return SelectorMatches(selector, SpecSelectors([]T{spec}))
}

func CanonicalGroups(groups []string, allowedGroups []string) ([]string, error) {
	out := make([]string, 0, len(groups))
	for _, group := range groups {
		if !contains(allowedGroups, group) {
			return nil, fmt.Errorf("unknown benchmark group: %s", group)
		}
		if !contains(out, group) {
			out = append(out, group)
		}
	}
	return out, nil
}

func DomainSpecs(root, group string) ([]Benchmark, error) {
	benchDir := filepath.Join(root, "benchmarks", group)
	names := make([]string, 0)
	seen := map[string]struct{}{}
	for _, name := range DefaultOrder {
		if fileExists(filepath.Join(benchDir, name+".leia")) {
			names = append(names, name)
			seen[name] = struct{}{}
		}
	}
	entries, err := os.ReadDir(benchDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	extras := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".leia" {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".leia")
		if _, ok := seen[name]; !ok {
			extras = append(extras, name)
		}
	}
	sort.Strings(extras)
	names = append(names, extras...)

	specs := make([]Benchmark, 0, len(names))
	for _, name := range names {
		if !EnabledInBuild(group, name) {
			continue
		}
		luajit := filepath.Join(root, "benchmarks", "lua_ref", group, name+".lua")
		if !fileExists(luajit) {
			luajit = ""
		}
		specs = append(specs, Benchmark{
			Group:  group,
			Name:   name,
			Leia:   filepath.Join(benchDir, name+".leia"),
			LuaJIT: luajit,
			Base:   RelatedBenchmarkBases[name],
		})
	}
	return specs, nil
}

func Discover(root string, groups []string) ([]Benchmark, error) {
	canonical, err := CanonicalGroups(groups, DomainGroups)
	if err != nil {
		return nil, err
	}
	var out []Benchmark
	for _, group := range canonical {
		specs, err := DomainSpecs(root, group)
		if err != nil {
			return nil, err
		}
		out = append(out, specs...)
	}
	return out, nil
}

func SelectSpecs[T Selectable](specs []T, selectors []string) ([]T, error) {
	if len(selectors) == 0 {
		return specs, nil
	}
	selected := make([]T, 0, len(selectors))
	seen := map[string]struct{}{}
	for _, raw := range selectors {
		id, ok := BenchmarkIDFromSelector(raw, DomainGroups)
		if !ok {
			return nil, fmt.Errorf("unknown benchmark selector: %s", raw)
		}
		found := false
		for _, spec := range specs {
			if spec.ID() != id {
				continue
			}
			found = true
			if _, already := seen[id]; !already {
				selected = append(selected, spec)
				seen[id] = struct{}{}
			}
		}
		if !found {
			return nil, fmt.Errorf("unknown benchmark selector: %s", raw)
		}
	}
	return selected, nil
}

func ResolveScriptPath(root, bench string, groups []string) (string, bool) {
	id, ok := BenchmarkIDFromSelector(bench, groups)
	if !ok {
		return "", false
	}
	parts := strings.SplitN(id, "/", 2)
	path := filepath.Join(root, "benchmarks", parts[0], parts[1]+".leia")
	if !fileExists(path) {
		return "", false
	}
	return path, true
}

func ResolveScriptIdentity(root, bench string, groups []string) (Benchmark, bool) {
	path, ok := ResolveScriptPath(root, bench, groups)
	if !ok {
		return Benchmark{}, false
	}
	group := filepath.Base(filepath.Dir(path))
	if !contains(groups, group) {
		return Benchmark{}, false
	}
	return Benchmark{Group: group, Name: strings.TrimSuffix(filepath.Base(path), ".leia"), Leia: path}, true
}

func GroupsForSelectors(root string, groups []string, selectors []string, allowedGroups []string) ([]string, error) {
	if groups == nil {
		groups = allowedGroups
	}
	out, err := CanonicalGroups(groups, allowedGroups)
	if err != nil {
		return nil, err
	}
	for _, selector := range selectors {
		identity, ok := ResolveScriptIdentity(root, selector, allowedGroups)
		if !ok {
			continue
		}
		if !contains(out, identity.Group) {
			out = append(out, identity.Group)
		}
	}
	return out, nil
}

func GroupsForSelection(root string, groups []string, selectors []string, allGroups bool, allowedGroups []string) ([]string, error) {
	if allGroups {
		return append([]string(nil), allowedGroups...), nil
	}
	return GroupsForSelectors(root, groups, selectors, allowedGroups)
}

func ParseSelectorCountOverrides(values []string, modes []string, optionName string) (map[[2]string]int, error) {
	overrides := map[[2]string]int{}
	for _, value := range values {
		key, rawCount, ok := strings.Cut(value, "=")
		if !ok {
			return nil, fmt.Errorf("%s entries must be DOMAIN/BENCH=N or MODE/DOMAIN/BENCH=N", optionName)
		}
		count, err := strconv.Atoi(rawCount)
		if err != nil {
			return nil, fmt.Errorf("invalid count in %q", value)
		}
		if count <= 0 {
			return nil, fmt.Errorf("count must be > 0")
		}
		mode := ""
		selector := key
		if head, tail, ok := strings.Cut(key, "/"); ok && contains(modes, head) {
			mode = head
			selector = tail
		}
		id, ok := BenchmarkIDFromSelector(selector, DomainGroups)
		if !ok {
			return nil, fmt.Errorf("%s selector must be a domain benchmark id or path: %q", optionName, selector)
		}
		overrides[[2]string{mode, id}] = count
	}
	return overrides, nil
}

func SelectorCountOverride(overrides map[[2]string]int, mode, benchmarkID string) (int, bool) {
	if value, ok := overrides[[2]string{mode, benchmarkID}]; ok {
		return value, true
	}
	value, ok := overrides[[2]string{"", benchmarkID}]
	return value, ok
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
