package leia_test

import (
	"os"
	"path/filepath"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestLLMReportArtifactContractHelper(t *testing.T) {
	for _, mode := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{leia.WithLibs(leia.LibLLM)}, mode.opts...)...)
			if err := vm.Exec(`
contract := llm.report_artifact_contract({name: "finrobot", version: "FR-GAP-012"})
section_ok, section_msg := llm.validate_output({
    id: "financial_analysis",
    title: "Financial Analysis",
    order: 1,
    required: true,
    content: "fixture-backed section",
    chart_refs: {"chart_revenue_ebitda"},
    source_refs: {"src_financial_metrics"},
    ai_disclosure: true,
    disclosure_ref: "ai_disclosure",
}, contract.schemas.report_section)
manifest_ok, manifest_msg := llm.validate_output({
    contract: "FR-GAP-012",
    report_id: "AAPL_2026_06_11_equity_report",
    generated_at: "2026-06-11T00:00:00Z",
    report_sections: {"financial_analysis"},
    chart_specs: {"chart_revenue_ebitda"},
    artifacts: {{id: "html", kind: "text/html", path: "report.html", status: "planned_not_rendered"}},
    source_annotations: {{
        id: "src_financial_metrics",
        title: "Financial metrics fixture",
        kind: "csv",
        locator: "financial_metrics_and_forecasts.csv",
        as_of: "2026-03-31",
        stale_after: "2026-06-30",
        stale: true,
        license: "fixture",
        retrieved_at: "2026-06-11T00:00:00Z",
        evidence_hash: "sha256:fixture",
    }},
    warnings: {"stale_data_warning: fixture data is stale"},
    ai_disclosure: "AI_DISCLOSURE: AI-assisted and source-backed.",
}, contract.schemas.artifact_manifest)
contract_version := contract.version
offline_verifiable := contract.offline_verifiable
renderer_required := contract.renderer_required
required_marker_count := #contract.required_markers
`); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			for name, want := range map[string]any{
				"contract_version":      "FR-GAP-012",
				"offline_verifiable":    true,
				"renderer_required":     false,
				"required_marker_count": int64(6),
				"section_ok":            true,
				"manifest_ok":           true,
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

func TestFinRobotReportArtifactContractExample(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "report_contract.leia"))
	if err != nil {
		t.Fatalf("ReadFile report_contract.leia: %v", err)
	}
	for _, mode := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{leia.WithLibs(leia.LibString | leia.LibLLM)}, mode.opts...)...)
			if err := vm.Exec(string(src)); err != nil {
				t.Fatalf("Exec report_contract.leia: %v", err)
			}
			for name, want := range map[string]any{
				"section_ok":  true,
				"chart_ok":    true,
				"source_ok":   true,
				"manifest_ok": true,
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
