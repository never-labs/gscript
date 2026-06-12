package leia_test

import (
	"fmt"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

type finrobotLivePackageSmokeResult struct {
	Summary string
	Fields  map[string]string
	Prints  []string
	Globals map[string]any
}

func runFinRobotLivePackageSummarySmoke(t *testing.T, path, summaryVar, summaryPrefix string, libs leia.LibFlags, extraGlobals ...string) []finrobotLivePackageSmokeResult {
	t.Helper()
	var results []finrobotLivePackageSmokeResult
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var prints []string
			vm := leia.New(append([]leia.Option{
				leia.WithLibs(libs),
				leia.WithPrint(func(args ...any) {
					parts := make([]string, 0, len(args))
					for _, arg := range args {
						parts = append(parts, fmt.Sprint(arg))
					}
					prints = append(prints, strings.Join(parts, " "))
				}),
			}, tc.opts...)...)
			if err := vm.ExecFile(path); err != nil {
				t.Fatalf("ExecFile: %v", err)
			}
			value, err := vm.Get(summaryVar)
			if err != nil {
				t.Fatalf("Get %s: %v", summaryVar, err)
			}
			summary, ok := value.(string)
			if !ok {
				t.Fatalf("%s = %T %#v, want string", summaryVar, value, value)
			}
			if len(prints) != 1 || prints[0] != summary {
				t.Fatalf("prints = %#v, want one summary print %q", prints, summary)
			}
			globals := map[string]any{}
			for _, name := range extraGlobals {
				value, err := vm.Get(name)
				if err != nil {
					t.Fatalf("Get %s: %v", name, err)
				}
				globals[name] = value
			}
			results = append(results, finrobotLivePackageSmokeResult{
				Summary: summary,
				Fields:  parseFinRobotLivePackageSummaryFields(t, summaryPrefix, summary),
				Prints:  append([]string(nil), prints...),
				Globals: globals,
			})
		})
	}
	return results
}

func parseFinRobotLivePackageSummaryFields(t *testing.T, prefix, value string) map[string]string {
	t.Helper()
	fields := strings.Fields(value)
	if len(fields) == 0 || fields[0] != prefix {
		t.Fatalf("unexpected summary prefix: %q", value)
	}
	result := map[string]string{}
	for _, field := range fields[1:] {
		parts := strings.SplitN(field, "=", 2)
		if len(parts) != 2 || parts[0] == "" {
			t.Fatalf("malformed summary field %q in %q", field, value)
		}
		result[parts[0]] = parts[1]
	}
	return result
}

func requireFinRobotSummaryFields(t *testing.T, summary map[string]string, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if summary[key] == "" {
			t.Fatalf("summary field %q missing in %#v", key, summary)
		}
	}
}

func finrobotSummaryPositiveCount(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return value != "0"
}
