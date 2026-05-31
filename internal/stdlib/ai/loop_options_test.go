package ai

import "testing"

func TestNormalizeLoopOptionsRequiresMessagesOrUser(t *testing.T) {
	_, err := NormalizeLoopOptions(LoopOptionInput{})
	if err == nil {
		t.Fatalf("expected validation error")
	}
}

func TestNormalizeLoopOptionsSynthesizesUserMessages(t *testing.T) {
	plan, err := NormalizeLoopOptions(LoopOptionInput{HasUser: true})
	if err != nil {
		t.Fatalf("NormalizeLoopOptions: %v", err)
	}
	if !plan.SynthesizeMessages {
		t.Fatalf("expected synthesized messages")
	}
}

func TestNormalizeLoopOptionsKeepsExplicitMessages(t *testing.T) {
	plan, err := NormalizeLoopOptions(LoopOptionInput{HasMessages: true, HasUser: true})
	if err != nil {
		t.Fatalf("NormalizeLoopOptions: %v", err)
	}
	if plan.SynthesizeMessages {
		t.Fatalf("did not expect synthesized messages")
	}
}

func TestNormalizeLoopOptionsDefaultsMaxSteps(t *testing.T) {
	plan, err := NormalizeLoopOptions(LoopOptionInput{HasUser: true, DefaultMaxSteps: 3})
	if err != nil {
		t.Fatalf("NormalizeLoopOptions: %v", err)
	}
	if !plan.SetDefaultMaxSteps || plan.DefaultMaxSteps != 3 {
		t.Fatalf("default max steps = (%v, %d), want (true, 3)", plan.SetDefaultMaxSteps, plan.DefaultMaxSteps)
	}
}

func TestNormalizeLoopOptionsDoesNotOverrideMaxSteps(t *testing.T) {
	plan, err := NormalizeLoopOptions(LoopOptionInput{HasUser: true, HasMaxSteps: true, DefaultMaxSteps: 3})
	if err != nil {
		t.Fatalf("NormalizeLoopOptions: %v", err)
	}
	if plan.SetDefaultMaxSteps {
		t.Fatalf("did not expect max_steps default")
	}
}

func TestNormalizeLoopOptionsStructuredOutputResponseFormat(t *testing.T) {
	plan, err := NormalizeLoopOptions(LoopOptionInput{HasUser: true, HasStructuredOutput: true})
	if err != nil {
		t.Fatalf("NormalizeLoopOptions: %v", err)
	}
	if !plan.SetJSONResponseFormat {
		t.Fatalf("expected JSON response format default")
	}
}

func TestNormalizeLoopOptionsKeepsResponseFormat(t *testing.T) {
	plan, err := NormalizeLoopOptions(LoopOptionInput{
		HasUser:             true,
		HasStructuredOutput: true,
		HasResponseFormat:   true,
	})
	if err != nil {
		t.Fatalf("NormalizeLoopOptions: %v", err)
	}
	if plan.SetJSONResponseFormat {
		t.Fatalf("did not expect JSON response format default")
	}
}

func TestNormalizeReactControls(t *testing.T) {
	got := NormalizeReactControls(0, -3, -4)
	if got.MaxSteps != int(DefaultReactMaxSteps) || got.MaxToolRetries != 0 || got.MaxHistoryTokens != 0 {
		t.Fatalf("NormalizeReactControls defaults = %+v", got)
	}

	got = NormalizeReactControls(3, 2, 128)
	if got.MaxSteps != 3 || got.MaxToolRetries != 2 || got.MaxHistoryTokens != 128 {
		t.Fatalf("NormalizeReactControls explicit = %+v", got)
	}
}
