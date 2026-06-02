package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
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
	llmRecord := fs.String("llm-record", "", "record LLM turns to a replay JSON file")
	llmReplay := fs.String("llm-replay", "", "replay LLM turns from a replay JSON file")
	updateGolden := fs.String("update-golden", "", "rewrite an LLM replay JSON file from a live evaluation run")
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

	report, err := evaluate.Run(evaluate.Options{
		Paths:               fs.Args(),
		Filter:              *filter,
		ListOnly:            *listOnly,
		LLMRecordPath:       *llmRecord,
		LLMReplayPath:       *llmReplay,
		LLMUpdateGoldenPath: *updateGolden,
		LLMProviderFactory:  cliDefaultLLMProviderFactory,
	})
	if err != nil {
		fmt.Fprintf(errw, "leia evaluate: %v\n", err)
		return 1
	}
	if *format == "text" {
		_, _ = io.WriteString(outw, evaluate.FormatText(report))
	} else {
		enc := json.NewEncoder(outw)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(errw, "leia evaluate: %v\n", err)
			return 1
		}
	}
	if report.Status == "failed" {
		return 1
	}
	return 0
}

func evaluateUsage(fs *flag.FlagSet) string {
	var b strings.Builder
	b.WriteString("usage: leia evaluate [options] [path-or-dir...]\n\n")
	b.WriteString("Run source-level evaluate blocks and emit a versioned agent evaluation report.\n\n")
	b.WriteString("Examples:\n")
	b.WriteString("  leia evaluate --format=text examples/evaluate/basic_assert.leia\n")
	b.WriteString("  leia evaluate --llm-replay examples/evaluate/agent_replay.records.json examples/evaluate/agent_replay.leia\n")
	b.WriteString("  leia evaluate --list --filter refund tests/agents\n\n")
	b.WriteString("LLM fixture modes are mutually exclusive:\n")
	b.WriteString("  --llm-replay       read a deterministic provider transcript and fail on drift\n")
	b.WriteString("  --llm-record       call the configured provider and save observed turns\n")
	b.WriteString("  --update-golden    call the configured provider and rewrite the fixture explicitly\n\n")
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
