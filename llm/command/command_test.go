package command

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/never-labs/gscript/llm"
)

func TestProviderTurnPassesModelAndRenderedPrompt(t *testing.T) {
	t.Setenv("GSCRIPT_LLM_COMMAND_HELPER", "1")
	provider := Provider{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperProcess", "--"},
		Model:   "mock-default",
	}

	res, err := provider.Turn(context.Background(), llm.TurnRequest{
		Messages: []llm.Message{
			{Role: "system", Text: "follow policy"},
			{Role: "user", Text: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("Turn returned error: %v", err)
	}
	if res.Status != "final_answer" {
		t.Fatalf("Status = %q, want final_answer", res.Status)
	}
	want := "--model|mock-default|System: follow policy\nUser: hello"
	if res.Text != want {
		t.Fatalf("Text = %q, want %q", res.Text, want)
	}
}

func TestProviderTurnKeepsExistingModelFlag(t *testing.T) {
	t.Setenv("GSCRIPT_LLM_COMMAND_HELPER", "1")
	provider := Provider{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperProcess", "--", "--model=preselected"},
	}

	res, err := provider.Turn(context.Background(), llm.TurnRequest{
		Model:    "ignored",
		Messages: []llm.Message{{Role: "user", Text: "hello"}},
	})
	if err != nil {
		t.Fatalf("Turn returned error: %v", err)
	}
	if strings.Contains(res.Text, "|ignored|") {
		t.Fatalf("Text = %q, should not append request model when args already include model flag", res.Text)
	}
	want := "--model=preselected|User: hello"
	if res.Text != want {
		t.Fatalf("Text = %q, want %q", res.Text, want)
	}
}

func TestProviderTurnRequiresCommand(t *testing.T) {
	_, err := Provider{}.Turn(context.Background(), llm.TurnRequest{})
	if err == nil || !strings.Contains(err.Error(), "llm command not configured") {
		t.Fatalf("err = %v, want command configuration error", err)
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GSCRIPT_LLM_COMMAND_HELPER") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			_, _ = os.Stdout.WriteString(strings.Join(os.Args[i+1:], "|"))
			os.Exit(0)
		}
	}
	os.Exit(2)
}
