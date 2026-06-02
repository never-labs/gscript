package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
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
	format := fs.String("format", "json", "output format: json or text")
	listOnly := fs.Bool("list", false, "list discovered evaluate cases without executing them")
	filter := fs.String("filter", "", "run only evaluate cases whose name, source path, or case id contains this text")
	record := fs.String("record", "", "alias for --llm-record")
	replay := fs.String("replay", "", "alias for --llm-replay")
	llmRecord := fs.String("llm-record", "", "record LLM turns to a replay JSON file")
	llmReplay := fs.String("llm-replay", "", "replay LLM turns from a replay JSON file")
	updateGolden := fs.String("update-golden", "", "rewrite an LLM replay JSON file from a live evaluation run")
	output := fs.String("output", "", "write the evaluate report to this file instead of stdout")
	if code, done := parseCLIFlags(fs, args); done {
		return code
	}
	if *jsonOut {
		*format = "json"
	}
	if *format != "json" && *format != "text" {
		fmt.Fprintf(errw, "leia evaluate: unknown format %q (want json or text)\n", *format)
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
	if *output != "" {
		if err := os.WriteFile(*output, rendered, 0o600); err != nil {
			fmt.Fprintf(errw, "leia evaluate: write %s: %v\n", *output, err)
			return 1
		}
	} else if _, err := outw.Write(rendered); err != nil {
		fmt.Fprintf(errw, "leia evaluate: %v\n", err)
		return 1
	}
	if report.Status == "failed" {
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
	if format == "text" {
		return []byte(evaluate.FormatText(report)), nil
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func evaluateUsage(fs *flag.FlagSet) string {
	var b strings.Builder
	b.WriteString("usage: leia evaluate [options] [path-or-dir...]\n\n")
	b.WriteString("Run source-level evaluate blocks and emit a versioned agent evaluation report.\n\n")
	b.WriteString("Examples:\n")
	b.WriteString("  leia evaluate --format=text examples/evaluate/basic_assert.leia\n")
	b.WriteString("  leia evaluate --json --output eval-report.json tests/agents\n")
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
