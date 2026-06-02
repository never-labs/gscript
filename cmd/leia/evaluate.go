package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/never-labs/leia/internal/tooling/evaluate"
)

func runEvaluateCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("evaluate", flag.ContinueOnError)
	fs.SetOutput(errw)
	jsonOut := fs.Bool("json", false, "write the evaluate report as JSON")
	format := fs.String("format", "json", "output format: json or text")
	llmRecord := fs.String("llm-record", "", "record LLM turns to a replay JSON file")
	llmReplay := fs.String("llm-replay", "", "replay LLM turns from a replay JSON file")
	updateGolden := fs.String("update-golden", "", "rewrite an LLM replay JSON file from a live evaluation run")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
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
