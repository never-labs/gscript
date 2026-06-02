package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/never-labs/leia/internal/tooling/evaluate"
)

func runEvaluateCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("evaluate", flag.ContinueOnError)
	fs.SetOutput(errw)
	fs.Usage = func() {
		_, _ = io.WriteString(errw, evaluateUsage(fs))
	}
	jsonOut := fs.Bool("json", false, "write the evaluate report as JSON")
	format := fs.String("format", "json", "output format: json, text, or html")
	listOnly := fs.Bool("list", false, "list discovered evaluate cases without executing them")
	filter := fs.String("filter", "", "run only evaluate cases whose name, source path, or case id contains this text")
	gate := fs.Bool("gate", false, "CI gate mode: exit non-zero when any selected case, replay check, or report finding fails")
	record := fs.String("record", "", "alias for --llm-record")
	replay := fs.String("replay", "", "alias for --llm-replay")
	llmRecord := fs.String("llm-record", "", "record LLM turns to a replay JSON file")
	llmReplay := fs.String("llm-replay", "", "replay LLM turns from a replay JSON file")
	updateGolden := fs.String("update-golden", "", "rewrite an LLM replay JSON file from a live evaluation run")
	output := fs.String("output", "", "write the evaluate report to this file instead of stdout")
	reportPathFlag := fs.String("report", "", "alias for --output")
	if code, done := parseCLIFlags(fs, args); done {
		return code
	}
	if *jsonOut {
		*format = "json"
	}
	if *format != "json" && *format != "text" && *format != "html" {
		fmt.Fprintf(errw, "leia evaluate: unknown format %q (want json, text, or html)\n", *format)
		return 2
	}
	reportPath, err := coalesceEvaluatePathFlag("output", *output, "report", *reportPathFlag)
	if err != nil {
		fmt.Fprintf(errw, "leia evaluate: %v\n", err)
		return 2
	}
	recordPath, err := coalesceEvaluatePathFlag("record", *record, "llm-record", *llmRecord)
	if err != nil {
		fmt.Fprintf(errw, "leia evaluate: %v\n", err)
		return 2
	}
	replayPath, err := coalesceEvaluatePathFlag("replay", *replay, "llm-replay", *llmReplay)
	if err != nil {
		fmt.Fprintf(errw, "leia evaluate: %v\n", err)
		return 2
	}

	report, err := evaluate.Run(evaluate.Options{
		Paths:               fs.Args(),
		Filter:              *filter,
		ListOnly:            *listOnly,
		LLMRecordPath:       recordPath,
		LLMReplayPath:       replayPath,
		LLMUpdateGoldenPath: *updateGolden,
		LLMProviderFactory:  cliDefaultLLMProviderFactory,
	})
	if err != nil {
		fmt.Fprintf(errw, "leia evaluate: %v\n", err)
		return 1
	}
	rendered, err := renderEvaluateReport(report, *format)
	if err != nil {
		fmt.Fprintf(errw, "leia evaluate: %v\n", err)
		return 1
	}
	if reportPath != "" {
		if err := os.WriteFile(reportPath, rendered, 0o600); err != nil {
			fmt.Fprintf(errw, "leia evaluate: write %s: %v\n", reportPath, err)
			return 1
		}
	} else if _, err := outw.Write(rendered); err != nil {
		fmt.Fprintf(errw, "leia evaluate: %v\n", err)
		return 1
	}
	if report.Status == "failed" || (*gate && report.Status != "ok") {
		return 1
	}
	return 0
}

func coalesceEvaluatePathFlag(shortName, shortValue, longName, longValue string) (string, error) {
	if shortValue == "" {
		return longValue, nil
	}
	if longValue == "" || longValue == shortValue {
		return shortValue, nil
	}
	return "", fmt.Errorf("--%s and --%s specify different files", shortName, longName)
}

func renderEvaluateReport(report evaluate.Report, format string) ([]byte, error) {
	switch format {
	case "text":
		return []byte(evaluate.FormatText(report)), nil
	case "html":
		return []byte(renderEvaluateHTML(report)), nil
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func renderEvaluateHTML(report evaluate.Report) string {
	statusClass := "ok"
	if report.Status != "ok" {
		statusClass = "failed"
	}
	var b strings.Builder
	b.WriteString("<!doctype html><html><head><meta charset=\"utf-8\">")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">")
	b.WriteString("<title>Leia Evaluate Report</title>")
	b.WriteString("<style>")
	b.WriteString("body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;margin:32px;color:#1f2937;background:#f8fafc}main{max-width:1120px;margin:auto}h1{font-size:28px;margin:0 0 8px}h2{font-size:18px;margin:28px 0 10px}.pill{display:inline-block;border-radius:999px;padding:4px 10px;font-size:13px;font-weight:600}.ok{background:#dcfce7;color:#166534}.failed{background:#fee2e2;color:#991b1b}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:10px;margin:20px 0}.card{background:white;border:1px solid #e5e7eb;border-radius:8px;padding:12px}.label{font-size:12px;color:#6b7280}.value{font-size:20px;font-weight:650;margin-top:4px}table{width:100%;border-collapse:collapse;background:white;border:1px solid #e5e7eb;border-radius:8px;overflow:hidden}th,td{text-align:left;padding:9px 10px;border-bottom:1px solid #eef2f7;font-size:14px}th{background:#f1f5f9;color:#475569}tr:last-child td{border-bottom:0}code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:13px}.muted{color:#64748b}.diag{color:#991b1b}.metric{white-space:nowrap}")
	b.WriteString("</style></head><body><main>")
	fmt.Fprintf(&b, "<h1>Leia Evaluate Report <span class=\"pill %s\">%s</span></h1>", statusClass, html.EscapeString(report.Status))
	fmt.Fprintf(&b, "<p class=\"muted\">started %s · schema %d · phase %s</p>", html.EscapeString(report.StartedAt), report.SchemaVersion, html.EscapeString(report.Phase))
	b.WriteString("<section class=\"grid\">")
	writeHTMLStat(&b, "Cases", fmt.Sprintf("%d/%d", report.Summary.CasesSelected, report.Summary.EvaluateBlocks))
	writeHTMLStat(&b, "Passed", fmt.Sprintf("%d", report.Summary.CasesPassed))
	writeHTMLStat(&b, "Failed", fmt.Sprintf("%d", report.Summary.CasesFailed))
	writeHTMLStat(&b, "Pass rate", fmt.Sprintf("%.2f", report.Summary.PassRate))
	writeHTMLStat(&b, "Duration", fmt.Sprintf("%dms", report.Summary.DurationMS))
	writeHTMLStat(&b, "Assertions", fmt.Sprintf("%d", report.Summary.Assertions))
	b.WriteString("</section>")
	if len(report.Metrics) > 0 {
		b.WriteString("<h2>Metrics</h2><table><thead><tr><th>Name</th><th>Type</th><th>Summary</th><th>Count</th></tr></thead><tbody>")
		for _, metric := range report.Metrics {
			fmt.Fprintf(&b, "<tr><td><code>%s</code></td><td>%s</td><td class=\"metric\">%s</td><td>%d</td></tr>",
				html.EscapeString(metric.Name),
				html.EscapeString(metric.Type),
				html.EscapeString(htmlMetricSummary(metric)),
				metric.Count,
			)
		}
		b.WriteString("</tbody></table>")
	}
	b.WriteString("<h2>Cases</h2><table><thead><tr><th>Status</th><th>Name</th><th>Location</th><th>Duration</th><th>Assertions</th><th>Metrics</th><th>Subcases</th></tr></thead><tbody>")
	for _, c := range report.Cases {
		fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td><td><code>%s:%d:%d</code></td><td>%dms</td><td>%d</td><td>%d</td><td>%d</td></tr>",
			html.EscapeString(c.Status),
			html.EscapeString(c.Name),
			html.EscapeString(c.SourcePath),
			c.Range.StartLine,
			c.Range.StartColumn,
			c.DurationMS,
			len(c.Assertions),
			len(c.Metrics),
			len(c.Subcases),
		)
		for _, subcase := range c.Subcases {
			fmt.Fprintf(&b, "<tr><td>↳ %s</td><td><code>%s</code></td><td></td><td>%dms</td><td></td><td>%d</td><td></td></tr>",
				html.EscapeString(subcase.Status),
				html.EscapeString(subcase.CaseID),
				subcase.DurationMS,
				len(subcase.Metrics),
			)
		}
	}
	b.WriteString("</tbody></table>")
	if len(report.Findings) > 0 {
		b.WriteString("<h2>Findings</h2><table><thead><tr><th>Severity</th><th>Kind</th><th>Location</th><th>Message</th></tr></thead><tbody>")
		for _, finding := range report.Findings {
			location := finding.Path
			if finding.Line > 0 {
				location = fmt.Sprintf("%s:%d:%d", finding.Path, finding.Line, finding.Column)
			}
			fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td><td><code>%s</code></td><td class=\"diag\">%s</td></tr>",
				html.EscapeString(finding.Severity),
				html.EscapeString(finding.Kind),
				html.EscapeString(location),
				html.EscapeString(finding.Message),
			)
		}
		b.WriteString("</tbody></table>")
	}
	b.WriteString("</main></body></html>\n")
	return b.String()
}

func writeHTMLStat(b *strings.Builder, label, value string) {
	fmt.Fprintf(b, "<div class=\"card\"><div class=\"label\">%s</div><div class=\"value\">%s</div></div>", html.EscapeString(label), html.EscapeString(value))
}

func htmlMetricSummary(metric evaluate.MetricSummary) string {
	switch metric.Type {
	case "bool":
		return fmt.Sprintf("pass_rate %.2f, true %d, false %d", metric.PassRate, metric.True, metric.False)
	case "number":
		return fmt.Sprintf("mean %.4g, min %.4g, max %.4g", metric.Mean, metric.Min, metric.Max)
	case "string":
		return formatEvaluateMetricValues(metric.Values)
	default:
		return ""
	}
}

func formatEvaluateMetricValues(values map[string]int) string {
	if len(values) == 0 {
		return "{}"
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("{")
	for i, key := range keys {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%s:%d", key, values[key])
	}
	b.WriteString("}")
	return b.String()
}

func evaluateUsage(fs *flag.FlagSet) string {
	var b strings.Builder
	b.WriteString("usage: leia evaluate [options] [path-or-dir...]\n\n")
	b.WriteString("Run source-level evaluate blocks and emit a versioned agent evaluation report.\n\n")
	b.WriteString("Examples:\n")
	b.WriteString("  leia evaluate --format=text examples/evaluate/basic_assert.leia\n")
	b.WriteString("  leia evaluate --json --report eval-report.json tests/agents\n")
	b.WriteString("  leia evaluate --format=html --report eval-report.html tests/agents\n")
	b.WriteString("  leia evaluate --replay examples/evaluate/agent_replay.records.json examples/evaluate/agent_replay.leia\n")
	b.WriteString("  leia evaluate --list --filter refund tests/agents\n\n")
	b.WriteString("LLM fixture modes are mutually exclusive:\n")
	b.WriteString("  --replay           read a deterministic provider transcript and fail on drift\n")
	b.WriteString("  --record           call the configured provider and save observed turns\n")
	b.WriteString("  --update-golden    call the configured provider and rewrite the fixture explicitly\n\n")
	b.WriteString("The explicit --llm-record and --llm-replay names are also accepted.\n\n")
	b.WriteString("Options:\n")
	fs.VisitAll(func(f *flag.Flag) {
		fmt.Fprintf(&b, "  -%s", f.Name)
		if f.DefValue != "false" && f.DefValue != "true" {
			b.WriteString(" VALUE")
		}
		fmt.Fprintf(&b, "\n      %s", f.Usage)
		if f.DefValue != "" && f.DefValue != "false" {
			fmt.Fprintf(&b, " (default %q)", f.DefValue)
		}
		b.WriteString("\n")
	})
	return b.String()
}
