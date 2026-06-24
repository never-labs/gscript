package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
)

func runBenchGateValidateCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("bench gate-validate", flag.ContinueOnError)
	fs.SetOutput(errw)
	kind := fs.String("kind", "compare", "Validation kind: compare or strict.")
	threshold := fs.Float64("threshold", 0.10, "Non-wall current/HEAD regression threshold.")
	wallThreshold := fs.Float64("wall-threshold", 0.25, "Wall-timed current/HEAD regression threshold.")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(errw, "usage: leia bench gate-validate --kind compare|strict [flags] ARTIFACT.json")
		return 2
	}
	data, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(errw, "bench gate-validate: %v\n", err)
		return 2
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		fmt.Fprintf(errw, "bench gate-validate: %v\n", err)
		return 2
	}
	switch *kind {
	case "compare", "timing":
		return validateBenchComparePayload(payload, *threshold, *wallThreshold, outw, errw)
	case "strict":
		return validateBenchStrictPayload(payload, outw, errw)
	default:
		fmt.Fprintf(errw, "bench gate-validate: unknown kind %q\n", *kind)
		return 2
	}
}

type benchCompareRankRow struct {
	Name          string
	Mode          string
	Current       *float64
	Head          *float64
	Change        *float64
	CurrentStatus string
	HeadStatus    string
	CurrentSource string
	HeadSource    string
	CurrentCV     *float64
	Note          string
}

type benchCompareViolation struct {
	Kind   string
	Name   string
	Mode   string
	Change float64
	Limit  float64
}

type benchCompareUnreliable struct {
	Name          string
	Mode          string
	CurrentStatus string
	HeadStatus    string
	CurrentSource string
	HeadSource    string
}

func validateBenchComparePayload(payload map[string]any, threshold, wallThreshold float64, outw, errw io.Writer) int {
	rows, ok := payload["results"].([]any)
	if !ok {
		fmt.Fprintln(errw, "performance gate: JSON artifact has no list-valued results")
		return 2
	}
	modes := benchGateStringList(payload["modes"])
	if len(modes) == 0 {
		modes = []string{"default"}
	}
	var ranked []benchCompareRankRow
	var violations []benchCompareViolation
	var unreliable []benchCompareUnreliable
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		group := benchGateString(row["group"])
		bench := benchGateString(row["benchmark"])
		name := bench
		if group != "" {
			name = group + "/" + bench
		}
		for _, mode := range modes {
			cur := benchGateSubject(row, mode, "current")
			head := benchGateSubject(row, mode, "head")
			curS := benchGateSeconds(cur)
			headS := benchGateSeconds(head)
			curStatus := benchGateStatus(cur)
			headStatus := benchGateStatus(head)
			curSource := benchGateString(cur["source"])
			headSource := benchGateString(head["source"])
			var ratio *float64
			var change *float64
			if curS != nil && headS != nil && *headS > 0 {
				r := *curS / *headS
				c := r - 1
				ratio = &r
				change = &c
			}
			_ = ratio
			notes := []string{}
			if curStatus == "low_resolution" || headStatus == "low_resolution" {
				notes = append(notes, "low_resolution")
				unreliable = append(unreliable, benchCompareUnreliable{name, mode, curStatus, headStatus, curSource, headSource})
			} else if curStatus == "ok" || curStatus == "partial" {
				if headStatus == "missing" && headS == nil {
					notes = append(notes, "current_only_new_benchmark")
				} else if headStatus != "ok" && headStatus != "partial" {
					notes = append(notes, "status="+curStatus+"/"+headStatus)
					unreliable = append(unreliable, benchCompareUnreliable{name, mode, curStatus, headStatus, curSource, headSource})
				}
			} else {
				notes = append(notes, "status="+curStatus+"/"+headStatus)
				unreliable = append(unreliable, benchCompareUnreliable{name, mode, curStatus, headStatus, curSource, headSource})
			}
			wall := benchGateIsWall(curSource) || benchGateIsWall(headSource)
			if wall {
				notes = append(notes, "wall_timed_startup_noise")
			}
			if change != nil && len(notes) == 0 && *change > threshold {
				violations = append(violations, benchCompareViolation{"regression", name, mode, *change, threshold})
			} else if change != nil && wall && (curStatus == "ok" || curStatus == "partial") && (headStatus == "ok" || headStatus == "partial") && *change > wallThreshold {
				violations = append(violations, benchCompareViolation{"wall_regression", name, mode, *change, wallThreshold})
			}
			note := "-"
			if len(notes) > 0 {
				note = strings.Join(notes, ",")
			}
			ranked = append(ranked, benchCompareRankRow{name, mode, curS, headS, change, curStatus, headStatus, curSource, headSource, benchGateCV(cur), note})
		}
	}
	sort.Slice(ranked, func(i, j int) bool {
		a, b := ranked[i].Change, ranked[j].Change
		if a == nil && b == nil {
			return ranked[i].Name < ranked[j].Name
		}
		if a == nil {
			return false
		}
		if b == nil {
			return true
		}
		return *a > *b
	})
	fmt.Fprintln(outw, "Performance gate current/HEAD ranking:")
	fmt.Fprintln(outw, "Benchmark                          Mode      Current      HEAD       Change    CV cur   Status        Source                 Note")
	fmt.Fprintln(outw, "-------------------------------------------------------------------------------------------------------------------------------")
	for _, row := range ranked {
		fmt.Fprintf(outw, "%-34s %-9s %10s %10s %9s %8s %-13s %-22s %s\n",
			row.Name, row.Mode, benchGateFmtSeconds(row.Current), benchGateFmtSeconds(row.Head), benchGateFmtChange(row.Change), benchGateFmtPct(row.CurrentCV),
			row.CurrentStatus+"/"+row.HeadStatus, row.CurrentSource+"/"+row.HeadSource, row.Note)
	}
	if len(violations) > 0 {
		fmt.Fprintln(outw, "\nPerformance gate violations:")
		fmt.Fprintln(outw, "Kind             Benchmark                          Mode      Change     Limit")
		fmt.Fprintln(outw, "----------------------------------------------------------------------------")
		for _, violation := range violations {
			fmt.Fprintf(outw, "%-16s %-34s %-9s %+8.2f%% %7.2f%%\n", violation.Kind, violation.Name, violation.Mode, violation.Change*100, violation.Limit*100)
		}
	}
	if len(unreliable) > 0 {
		fmt.Fprintln(outw, "\nUnreliable timing rows:")
		fmt.Fprintln(outw, "Benchmark                          Mode      Current/HEAD status     Current/HEAD source")
		fmt.Fprintln(outw, "----------------------------------------------------------------------------------------")
		for _, row := range unreliable {
			fmt.Fprintf(outw, "%-34s %-9s %-23s %s\n", row.Name, row.Mode, row.CurrentStatus+"/"+row.HeadStatus, row.CurrentSource+"/"+row.HeadSource)
		}
	}
	if len(violations) > 0 || len(unreliable) > 0 {
		return 1
	}
	fmt.Fprintln(outw, "\nPerformance gate passed.")
	return 0
}

func validateBenchStrictPayload(payload map[string]any, outw, errw io.Writer) int {
	rows, ok := payload["results"].([]any)
	if !ok {
		fmt.Fprintln(errw, "strict gate: JSON artifact has no list-valued results")
		return 2
	}
	type violation struct{ name, mode, status, checksum string }
	var violations []violation
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		group := benchGateString(row["group"])
		bench := benchGateString(row["benchmark"])
		name := bench
		if group != "" && !strings.HasPrefix(bench, group+"/") {
			name = group + "/" + bench
		}
		modes, ok := row["modes"].(map[string]any)
		if !ok {
			violations = append(violations, violation{name, "-", "missing_modes", "-"})
			continue
		}
		for _, mode := range []string{"vm", "default", "no_filter"} {
			result, ok := modes[mode].(map[string]any)
			if !ok {
				violations = append(violations, violation{name, mode, "missing", "-"})
				continue
			}
			status := benchGateStatus(result)
			checksum := benchGateString(result["checksum_status"])
			if status != "ok" {
				violations = append(violations, violation{name, mode, status, benchGateDefault(checksum, "-")})
			} else if checksum != "" && checksum != "ok" && checksum != "single" {
				violations = append(violations, violation{name, mode, status, checksum})
			}
		}
	}
	if len(violations) > 0 {
		fmt.Fprintln(outw, "Strict gate violations:")
		fmt.Fprintln(outw, "Benchmark                          Mode       Status           Checksum")
		fmt.Fprintln(outw, "-----------------------------------------------------------------------")
		for _, row := range violations {
			fmt.Fprintf(outw, "%-34s %-10s %-16s %s\n", row.name, row.mode, row.status, row.checksum)
		}
		return 1
	}
	fmt.Fprintln(outw, "Strict gate passed.")
	return 0
}

func benchGateSubject(row map[string]any, mode, name string) map[string]any {
	modes, ok := row["modes"].(map[string]any)
	if !ok {
		return map[string]any{}
	}
	modeRow, ok := modes[mode].(map[string]any)
	if !ok {
		return map[string]any{}
	}
	subject, ok := modeRow[name].(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return subject
}

func benchGateSeconds(item map[string]any) *float64 {
	if stats, ok := item["stats"].(map[string]any); ok {
		if value, ok := benchGateFloat(stats["median"]); ok {
			return &value
		}
	}
	if value, ok := benchGateFloat(item["seconds"]); ok {
		return &value
	}
	return nil
}

func benchGateCV(item map[string]any) *float64 {
	stats, ok := item["stats"].(map[string]any)
	if !ok {
		return nil
	}
	if value, ok := benchGateFloat(stats["cv_pct"]); ok {
		return &value
	}
	return nil
}

func benchGateFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		if math.IsNaN(v) {
			return 0, false
		}
		return v, true
	case int:
		return float64(v), true
	default:
		return 0, false
	}
}

func benchGateString(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func benchGateStatus(item map[string]any) string {
	status := benchGateString(item["status"])
	if status == "" {
		return "missing"
	}
	return status
}

func benchGateStringList(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, benchGateString(item))
	}
	return out
}

func benchGateIsWall(source string) bool {
	parts := strings.Split(source, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "wall_repeat" || part == "wall_hr" {
			return true
		}
	}
	return false
}

func benchGateFmtSeconds(value *float64) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%.6fs", *value)
}

func benchGateFmtChange(value *float64) string {
	if value == nil || math.IsNaN(*value) {
		return "-"
	}
	return fmt.Sprintf("%+.2f%%", *value*100)
}

func benchGateFmtPct(value *float64) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%.2f%%", *value)
}

func benchGateDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
