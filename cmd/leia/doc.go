package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var docExecCommand = exec.Command

func runDocCommand(args []string, outw, errw io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errw, "usage: leia doc [generate|check] [flags]")
		return 2
	}
	switch args[0] {
	case "generate":
		return runDocGenerateCommand(args[1:], outw, errw)
	case "check":
		return runDocCheckCommand(args[1:], outw, errw)
	case "help", "-h", "--help":
		fmt.Fprintln(outw, "usage: leia doc [generate|check] [flags]")
		return 0
	default:
		fmt.Fprintf(errw, "leia doc: unknown mode %q (want generate or check)\n", args[0])
		return 2
	}
}

func runDocGenerateCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("doc generate", flag.ContinueOnError)
	fs.SetOutput(errw)
	outputDir := fs.String("output", "", "write generated docs to a directory")
	layout := fs.String("layout", "flat", "output layout: flat or site")
	format := fs.String("format", "markdown", "output format: markdown or json")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if len(fs.Args()) != 0 {
		fmt.Fprintln(errw, "usage: leia doc generate [--output DIR] [--layout flat|site] [--format markdown|json]")
		return 2
	}
	if *layout != "flat" && *layout != "site" {
		fmt.Fprintf(errw, "leia doc generate: unknown layout %q (want flat or site)\n", *layout)
		return 2
	}
	if *format != "markdown" && *format != "json" {
		fmt.Fprintf(errw, "leia doc generate: unknown format %q (want markdown or json)\n", *format)
		return 2
	}

	cliDoc := generateCLIReferenceMarkdown()
	stdlibDoc := generateStdlibInventoryMarkdown()
	dialectDoc := generateDialectReferenceMarkdown()
	if *format == "json" {
		cliDoc = generateCLIReferenceJSON()
		stdlibDoc = generateStdlibInventoryJSON()
		dialectDoc = generateDialectReferenceJSON()
	}
	if *outputDir == "" {
		if *format == "json" {
			if _, err := outw.Write(generateCombinedReferenceJSON()); err != nil {
				fmt.Fprintf(errw, "leia doc generate: %v\n", err)
				return 1
			}
			return 0
		}
		if _, err := outw.Write(cliDoc); err != nil {
			fmt.Fprintf(errw, "leia doc generate: %v\n", err)
			return 1
		}
		if _, err := outw.Write([]byte("\n")); err != nil {
			fmt.Fprintf(errw, "leia doc generate: %v\n", err)
			return 1
		}
		if _, err := outw.Write(stdlibDoc); err != nil {
			fmt.Fprintf(errw, "leia doc generate: %v\n", err)
			return 1
		}
		if _, err := outw.Write([]byte("\n")); err != nil {
			fmt.Fprintf(errw, "leia doc generate: %v\n", err)
			return 1
		}
		if _, err := outw.Write(dialectDoc); err != nil {
			fmt.Fprintf(errw, "leia doc generate: %v\n", err)
			return 1
		}
		return 0
	}
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		fmt.Fprintf(errw, "leia doc generate: %v\n", err)
		return 1
	}
	files := generatedDocFiles(*layout, *format, cliDoc, stdlibDoc, dialectDoc)
	for name, content := range files {
		path := filepath.Join(*outputDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			fmt.Fprintf(errw, "leia doc generate: %v\n", err)
			return 1
		}
		if err := os.WriteFile(path, content, 0644); err != nil {
			fmt.Fprintf(errw, "leia doc generate: %v\n", err)
			return 1
		}
		fmt.Fprintln(outw, path)
	}
	return 0
}

func generatedDocFiles(layout, format string, cliDoc, stdlibDoc, dialectDoc []byte) map[string][]byte {
	ext := "md"
	if format == "json" {
		ext = "json"
	}
	if layout == "site" {
		return map[string][]byte{
			filepath.Join("reference", "cli", "index."+ext):      cliDoc,
			filepath.Join("reference", "stdlib", "index."+ext):   stdlibDoc,
			filepath.Join("reference", "dialects", "index."+ext): dialectDoc,
		}
	}
	return map[string][]byte{
		"cli." + ext:      cliDoc,
		"stdlib." + ext:   stdlibDoc,
		"dialects." + ext: dialectDoc,
	}
}

type docCLIReference struct {
	SchemaVersion int             `json:"schema_version"`
	Commands      []docCLICommand `json:"commands"`
}

type docCLICommand struct {
	Name    string `json:"name"`
	Usage   string `json:"usage"`
	Summary string `json:"summary"`
}

type docStdlibInventory struct {
	SchemaVersion int              `json:"schema_version"`
	Layers        []cliStdlibLayer `json:"layers"`
}

type docDialectReference struct {
	SchemaVersion int                    `json:"schema_version"`
	Dialects      []cliDialectCapability `json:"dialects"`
}

type docReferenceBundle struct {
	SchemaVersion int                 `json:"schema_version"`
	CLI           docCLIReference     `json:"cli"`
	Stdlib        docStdlibInventory  `json:"stdlib"`
	Dialects      docDialectReference `json:"dialects"`
}

func runDocCheckCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("doc check", flag.ContinueOnError)
	fs.SetOutput(errw)
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if len(fs.Args()) != 0 {
		fmt.Fprintln(errw, "usage: leia doc check")
		return 2
	}
	script, err := findScriptFromCWD(filepath.Join("scripts", "docs_check.sh"))
	if err != nil {
		fmt.Fprintf(errw, "leia doc check: %v\n", err)
		return 1
	}
	cmd := docExecCommand("bash", script)
	cmd.Stdout = outw
	cmd.Stderr = errw
	cmd.Dir = filepath.Dir(filepath.Dir(script))
	return runExternalCommand(cmd, "leia doc check", errw)
}

func generateCLIReferenceMarkdown() []byte {
	var b bytes.Buffer
	fmt.Fprintln(&b, "# Leia CLI Reference")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Generated from the current `leia` binary capabilities.")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "| Command | Usage | Summary |")
	fmt.Fprintln(&b, "|---|---|---|")
	for _, command := range cliReferenceData() {
		fmt.Fprintf(&b, "| `%s` | `%s` | %s |\n", command.Name, command.Usage, command.Summary)
	}
	return b.Bytes()
}

func generateCLIReferenceJSON() []byte {
	return marshalGeneratedDoc(docCLIReference{
		SchemaVersion: 1,
		Commands:      cliReferenceData(),
	})
}

func generateStdlibInventoryMarkdown() []byte {
	caps := buildCapabilities()
	var b bytes.Buffer
	fmt.Fprintln(&b, "# Leia Standard Library Inventory")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "This inventory is generated from the standard-library metadata used by the current `leia` binary.")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "| Layer | Module | Description | Safe default | Capabilities |")
	fmt.Fprintln(&b, "|---|---|---|---|---|")
	for _, layer := range caps.StdlibLayers {
		for _, module := range layer.Modules {
			capabilities := "none"
			if len(module.Capabilities) > 0 {
				capabilities = strings.Join(module.Capabilities, ", ")
			}
			fmt.Fprintf(&b, "| `%s` | `%s` | %s | %t | %s |\n", layer.Name, strings.TrimSpace(module.Name), module.Description, module.SafeDefault, capabilities)
		}
	}
	return b.Bytes()
}

func generateStdlibInventoryJSON() []byte {
	return marshalGeneratedDoc(docStdlibInventory{
		SchemaVersion: 1,
		Layers:        buildStdlibLayerCapabilities(),
	})
}

func generateDialectReferenceMarkdown() []byte {
	caps := buildCapabilities()
	var b bytes.Buffer
	fmt.Fprintln(&b, "# Leia Tagged Dialects")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "The registry table is generated from the current `leia` binary dialect registry; explanatory sections below are maintained with the generated reference.")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Leia supports DSL-native tagged dialects for compact host automation, data format handling, web routing, q-style analytics, spreadsheets, and optional LLM integrations. A dialect is an explicit tagged expression that returns an ordinary Leia value.")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Forms")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "```leia")
	fmt.Fprintln(&b, "status := sh`git status --short`")
	fmt.Fprintln(&b, "checked := sh!`printf checked`")
	fmt.Fprintln(&b, "argv_checked := cmd!`printf checked`")
	fmt.Fprintln(&b, "out := $`printf hello`")
	fmt.Fprintln(&b, "files := glob`examples/**/*.leia`")
	fmt.Fprintln(&b, "data := json`{\"name\": ${name}}`")
	fmt.Fprintln(&b, "summarizer := agent {")
	fmt.Fprintln(&b, "    name: \"summary_agent\"")
	fmt.Fprintln(&b, "    config: func(summary) {")
	fmt.Fprintln(&b, "        return {model: \"example-model\", user: summary}, nil")
	fmt.Fprintln(&b, "    }")
	fmt.Fprintln(&b, "    params: [\"summary\"]")
	fmt.Fprintln(&b, "}")
	fmt.Fprintln(&b, "```")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "`${expr}` is the interpolation form inside tagged strings. Each dialect decides how interpolated values are encoded. `tag!` is the fail-fast form for dialects that support recoverable errors; `sh!` and `cmd!` raise on non-zero command exits while preserving the same result shape for successful commands.")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Built-In Dialects")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "| Tag | Category | Eval | Block | Capabilities | Aliases |")
	fmt.Fprintln(&b, "|---|---|---|---|---|---|")
	for _, dialect := range caps.Dialects {
		capabilities := "none"
		if len(dialect.Capabilities) > 0 {
			capabilities = strings.Join(dialect.Capabilities, ", ")
		}
		aliases := "none"
		if len(dialect.Aliases) > 0 {
			aliases = strings.Join(dialect.Aliases, ", ")
		}
		fmt.Fprintf(&b, "| `%s` | `%s` | %t | %t | %s | %s |\n", dialect.Name, dialect.Category, dialect.Eval, dialect.Block, capabilities, aliases)
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Capability Categories")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "- Host automation tags such as `sh`, `cmd`, `glob`, and `env` use host filesystem, process, or environment capabilities.")
	fmt.Fprintln(&b, "- Web and network-facing tags such as `serve` must be denied when the embedding host has not granted the relevant network capability.")
	fmt.Fprintln(&b, "- Optional LLM tags such as `model`, `turn`, `tool`, and `agent` use the same `llm.turn` policy as the `llm` standard library.")
	fmt.Fprintln(&b, "- Pure text, protocol, and data tags return ordinary values and should be documented with runnable examples before being treated as stable public surface.")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Important Result Shapes")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "| Dialect | Result shape |")
	fmt.Fprintln(&b, "|---|---|")
	fmt.Fprintln(&b, "| `sh`, `$` | Command result table with `ok`, `code`, `stdout`, `stderr`, and `text`. |")
	fmt.Fprintln(&b, "| `cmd` | Argv-safe command result table with the same command result shape as `sh`. |")
	fmt.Fprintln(&b, "| `glob` | Sorted path array. |")
	fmt.Fprintln(&b, "| `sql` | `{query, args, names}` with named parameters lowered to positional placeholders. |")
	fmt.Fprintln(&b, "| `q` | Symbolic q-style vectors, dictionaries, and tables. SoA rollups use `q.query(soa, plan_table)`. |")
	fmt.Fprintln(&b, "| `xlsx` encode | Workbook byte string suitable for writing or decoding with `excel`. |")
	fmt.Fprintln(&b, "| `excel` decode | Row array; with `{headers: true}`, rows are tables keyed by the first worksheet row. |")
	fmt.Fprintln(&b, "| `serve` | Route server descriptor/loopback result as documented by runnable web examples. |")
	fmt.Fprintln(&b, "| `turn` | `(result, err)` for a single LLM provider request. |")
	fmt.Fprintln(&b, "| `agent` | Callable agent value. |")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Examples")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "```bash")
	fmt.Fprintln(&b, "go run ./cmd/leia examples check examples/hello/dialects.leia examples/dialects/text_parsing.leia")
	fmt.Fprintln(&b, "go run ./cmd/leia examples run repo-dialects-data_aggregation_report")
	fmt.Fprintln(&b, "```")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "The larger tooling workflow combines shell/process dialects, SQLite frames, q-style aggregation, spreadsheet round-tripping, an LLM-style agent boundary, and a loopback web route. The focused examples below show the same dialect surface in smaller programs.")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Additional focused evidence lives in:")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "- `examples/dialects/ai_prompt_quote.leia` for prompt and quote tags.")
	fmt.Fprintln(&b, "- `examples/dialects/data_aggregation_report.leia` for mixed data-format tags.")
	fmt.Fprintln(&b, "- `examples/dialects/http_protocol_trace.leia` and `examples/dialects/network_protocols.leia` for web/protocol tags.")
	fmt.Fprintln(&b, "- `examples/data/q_trade_analytics_project/main.leia` for q-style vectors, dictionaries, scans, filters, and table rollups.")
	fmt.Fprintln(&b, "- `examples/web/serve_dialect_app.leia` and `examples/web/route_workbench.leia` for route/server evidence.")
	return b.Bytes()
}

func generateDialectReferenceJSON() []byte {
	return marshalGeneratedDoc(docDialectReference{
		SchemaVersion: 1,
		Dialects:      buildDialectCapabilities(),
	})
}

func generateCombinedReferenceJSON() []byte {
	return marshalGeneratedDoc(docReferenceBundle{
		SchemaVersion: 1,
		CLI: docCLIReference{
			SchemaVersion: 1,
			Commands:      cliReferenceData(),
		},
		Stdlib: docStdlibInventory{
			SchemaVersion: 1,
			Layers:        buildStdlibLayerCapabilities(),
		},
		Dialects: docDialectReference{
			SchemaVersion: 1,
			Dialects:      buildDialectCapabilities(),
		},
	})
}

func cliReferenceData() []docCLICommand {
	topics := cliHelpTopics()
	commands := cliCommandNames()
	out := make([]docCLICommand, 0, len(commands))
	for _, command := range commands {
		topic := topics[command]
		out = append(out, docCLICommand{
			Name:    command,
			Usage:   topic.Usage,
			Summary: topic.Summary,
		})
	}
	return out
}

func marshalGeneratedDoc(v any) []byte {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(fmt.Sprintf("marshal generated docs: %v", err))
	}
	return append(data, '\n')
}
