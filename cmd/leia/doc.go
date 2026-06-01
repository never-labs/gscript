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
	if *format == "json" {
		cliDoc = generateCLIReferenceJSON()
		stdlibDoc = generateStdlibInventoryJSON()
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
		return 0
	}
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		fmt.Fprintf(errw, "leia doc generate: %v\n", err)
		return 1
	}
	files := generatedDocFiles(*layout, *format, cliDoc, stdlibDoc)
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

func generatedDocFiles(layout, format string, cliDoc, stdlibDoc []byte) map[string][]byte {
	ext := "md"
	if format == "json" {
		ext = "json"
	}
	if layout == "site" {
		return map[string][]byte{
			filepath.Join("reference", "cli", "index."+ext):    cliDoc,
			filepath.Join("reference", "stdlib", "index."+ext): stdlibDoc,
		}
	}
	return map[string][]byte{
		"cli." + ext:    cliDoc,
		"stdlib." + ext: stdlibDoc,
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

type docReferenceBundle struct {
	SchemaVersion int                `json:"schema_version"`
	CLI           docCLIReference    `json:"cli"`
	Stdlib        docStdlibInventory `json:"stdlib"`
}

func runDocCheckCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("doc check", flag.ContinueOnError)
	fs.SetOutput(errw)
	if err := fs.Parse(args); err != nil {
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
	fmt.Fprintln(&b, "| Command | Summary |")
	fmt.Fprintln(&b, "|---|---|")
	for _, command := range cliReferenceData() {
		fmt.Fprintf(&b, "| `%s` | %s |\n", command.Name, command.Summary)
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
	fmt.Fprintln(&b, "Generated from the current runtime stdlib catalog.")
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
