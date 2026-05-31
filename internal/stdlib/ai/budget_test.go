package ai

import "testing"

func TestDefaultBudgetLimits(t *testing.T) {
	got := DefaultBudgetLimits()
	if got.MaxTokens != -1 || got.MaxTurns != -1 || got.MaxCalls != -1 || got.MaxMoney != -1 || got.MaxTimeSeconds != -1 {
		t.Fatalf("default budget limits = %+v", got)
	}
}

func TestNormalizeBudgetLimits(t *testing.T) {
	got := NormalizeBudgetLimits(BudgetLimits{
		MaxTokens:      -5,
		MaxTurns:       0,
		MaxCalls:       2,
		MaxMoney:       -0.5,
		MaxTimeSeconds: -1,
	})
	if got.MaxTokens != -1 || got.MaxTurns != 0 || got.MaxCalls != 2 || got.MaxMoney != -1 || got.MaxTimeSeconds != -1 {
		t.Fatalf("normalized budget limits = %+v", got)
	}
}
