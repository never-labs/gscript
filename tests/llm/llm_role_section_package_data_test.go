package leia_test

import (
	"path/filepath"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestFinRobotRoleProfilePackageData(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{leia.WithLibs(leia.LibLLM)}, mode.opts...)...)
			if err := vm.ExecFile(filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "role_profiles.leia")); err != nil {
				t.Fatalf("ExecFile role_profiles.leia: %v", err)
			}
			for name, want := range map[string]any{
				"role_registry_count":     int64(4),
				"market_role_ok":          true,
				"section_writer_ok":       true,
				"section_writer_contract": "report_section_payload_v1",
				"market_prompt_snapshot":  "As a Market Analyst, your responsibilities are as follows:\n - Collect local market evidence from replayable fixtures.\n - Summarize market context and source coverage for the report team.\n - Return a structured handoff and the TERMINATE convention when complete.\n\nReply \"TERMINATE\" in the end when everything is done.",
			} {
				got, err := vm.Get(name)
				if err != nil {
					t.Fatalf("Get %s: %v", name, err)
				}
				if got != want {
					t.Fatalf("%s = %#v, want %#v", name, got, want)
				}
			}
		})
	}
}

func TestFinRobotReportSectionPackageData(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{leia.WithLibs(leia.LibLLM)}, mode.opts...)...)
			if err := vm.ExecFile(filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "section_agents.leia")); err != nil {
				t.Fatalf("ExecFile section_agents.leia: %v", err)
			}
			for name, want := range map[string]any{
				"taxonomy_count":             int64(5),
				"taxonomy_signature":         "company_overview>financial_analysis>valuation_analysis>news_and_catalysts>risk_and_competitors",
				"overview_ok":                true,
				"financial_ok":               true,
				"valuation_ok":               true,
				"news_catalyst_ok":           true,
				"risk_competitor_ok":         true,
				"valuation_schema_methods":   int64(1),
				"risk_competitor_peer_count": int64(3),
			} {
				got, err := vm.Get(name)
				if err != nil {
					t.Fatalf("Get %s: %v", name, err)
				}
				if got != want {
					t.Fatalf("%s = %#v, want %#v", name, got, want)
				}
			}
		})
	}
}
