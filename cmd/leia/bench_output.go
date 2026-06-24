package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var benchOutputTimeRE = regexp.MustCompile(`(?m)^Time:\s*([0-9]+(?:\.[0-9]+)?)s\b`)

type benchTextCommandResult struct {
	Output      string
	Status      string
	ExitCode    *int
	WallSeconds float64
}

type benchModeCommand struct {
	Args        []string
	Env         []string
	Unavailable string
}

func benchParseTime(output string) *float64 {
	match := benchOutputTimeRE.FindStringSubmatch(output)
	if match == nil {
		return nil
	}
	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return nil
	}
	return &value
}

func benchParseCounter(pattern *regexp.Regexp, output string) int {
	match := pattern.FindStringSubmatch(output)
	if match == nil {
		return 0
	}
	value, err := strconv.Atoi(match[1])
	if err != nil {
		return 0
	}
	return value
}

func benchOutputTail(output string, limit int) string {
	if limit <= 0 {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	nonEmpty := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmpty = append(nonEmpty, line)
		}
	}
	if len(nonEmpty) > limit {
		nonEmpty = nonEmpty[len(nonEmpty)-limit:]
	}
	return strings.Join(nonEmpty, "\n")
}

func benchTextOutput(output []byte) string {
	if output == nil {
		return ""
	}
	return string(bytes.ToValidUTF8(output, []byte("\uFFFD")))
}

func benchRunTextCommand(args []string, timeout time.Duration, env []string) benchTextCommandResult {
	started := time.Now()
	if len(args) == 0 {
		return benchTextCommandResult{Status: "error", ExitCode: intPtr(1)}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := benchExecCommand(args[0], args[1:]...)
	if env != nil {
		cmd.Env = env
	}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Start()
	if err != nil {
		return benchTextCommandResult{Output: output.String(), Status: "error", ExitCode: intPtr(1), WallSeconds: time.Since(started).Seconds()}
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err = <-done:
	case <-ctx.Done():
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		<-done
		text := output.String() + fmt.Sprintf("\nTIMEOUT after %gs", timeout.Seconds())
		return benchTextCommandResult{Output: text, Status: "timeout", WallSeconds: time.Since(started).Seconds()}
	}
	if err != nil {
		exitCode := 1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
		return benchTextCommandResult{Output: output.String(), Status: "error", ExitCode: &exitCode, WallSeconds: time.Since(started).Seconds()}
	}
	return benchTextCommandResult{Output: output.String(), Status: "ok", ExitCode: intPtr(0), WallSeconds: time.Since(started).Seconds()}
}

func benchBuildLeia(root, out, failureMessage string, errw io.Writer) error {
	cmd := benchExecCommand("go", "build", "-o", out, "./cmd/leia")
	cmd.Dir = root
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		if errw != nil {
			_, _ = io.WriteString(errw, output.String())
		}
		exitCode := 1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
		message := failureMessage
		if message == "" {
			message = "build failed with exit {exit_code}"
		}
		message = strings.ReplaceAll(message, "{root}", root)
		message = strings.ReplaceAll(message, "{exit_code}", strconv.Itoa(exitCode))
		return errors.New(message)
	}
	return nil
}

func benchMarkdownRow(cells ...any) string {
	parts := make([]string, len(cells))
	for i, cell := range cells {
		parts[i] = fmt.Sprint(cell)
	}
	return "| " + strings.Join(parts, " | ") + " |"
}

func benchMarkdownSection(title, header, separator string) []string {
	lines := []string{"", "## " + title, ""}
	if header != "" {
		lines = append(lines, header)
		if separator != "" {
			lines = append(lines, separator)
		}
	}
	return lines
}

func benchLeiaModeCommand(mode, leiaBin, script string) ([]string, []string, error) {
	env := os.Environ()
	switch mode {
	case "vm":
		return []string{leiaBin, "-vm", script}, env, nil
	case "default":
		return []string{leiaBin, "-jit", "-jit-stats", "-exit-stats", script}, env, nil
	case "no_filter":
		return []string{leiaBin, "-jit", "-jit-stats", "-exit-stats", script}, append(env, "LEIA_TIER2_NO_FILTER=1"), nil
	default:
		return nil, nil, fmt.Errorf("unknown mode: %s", mode)
	}
}

func benchBenchmarkModeCommand(mode, leiaBin, leiaScript, luajitBin, luajitScript string) (benchModeCommand, error) {
	if mode == "luajit" {
		if luajitBin == "" {
			return benchModeCommand{Unavailable: "skipped"}, nil
		}
		if luajitScript == "" || !benchFileExists(luajitScript) {
			return benchModeCommand{Unavailable: "missing"}, nil
		}
		return benchModeCommand{Args: []string{luajitBin, luajitScript}}, nil
	}
	if leiaBin == "" || leiaScript == "" || !benchFileExists(leiaScript) {
		return benchModeCommand{Unavailable: "missing"}, nil
	}
	args, env, err := benchLeiaModeCommand(mode, leiaBin, leiaScript)
	if err != nil {
		return benchModeCommand{}, err
	}
	return benchModeCommand{Args: args, Env: env}, nil
}

func benchFileExists(path string) bool {
	info, err := os.Stat(filepath.Clean(path))
	return err == nil && !info.IsDir()
}
