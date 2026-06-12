package leia_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenericAIManifestNamingAndDialectFieldConvergence(t *testing.T) {
	for _, tc := range []struct {
		dir               string
		packageName       string
		dialectKeys       []string
		allowCapabilityID bool
	}{
		{
			dir:         "generic_model_registry",
			packageName: "leia-generic-ai-model-registry",
			dialectKeys: []string{
				"dialect_capability_id",
			},
		},
		{
			dir:         "generic_turn_runner",
			packageName: "leia-generic-ai-turn-runner",
		},
		{
			dir:         "generic_agent_runner",
			packageName: "leia-generic-ai-agent-runner",
			dialectKeys: []string{
				"dialect_backend_shape",
			},
		},
		{
			dir:               "generic_analytical_model_contracts",
			packageName:       "leia-generic-ai-analytical-model-contracts",
			allowCapabilityID: true,
		},
		{
			dir:               "generic_agent_state_store",
			packageName:       "leia-generic-ai-agent-state-store",
			allowCapabilityID: true,
		},
		{
			dir:               "generic_evidence_report_artifacts",
			packageName:       "leia-generic-ai-evidence-report-artifacts",
			allowCapabilityID: true,
		},
		{
			dir:               "generic_evidence_verification",
			packageName:       "leia-generic-ai-evidence-verification",
			allowCapabilityID: true,
		},
		{
			dir:               "generic_coding_workspace",
			packageName:       "leia-generic-ai-coding-workspace",
			allowCapabilityID: true,
		},
		{
			dir:               "generic_data_normalization_contracts",
			packageName:       "leia-generic-ai-data-normalization-contracts",
			allowCapabilityID: true,
		},
		{
			dir:               "generic_chart_render_contracts",
			packageName:       "leia-generic-ai-chart-render-contracts",
			allowCapabilityID: true,
		},
		{
			dir:               "generic_data_provider_boundary",
			packageName:       "leia-generic-ai-data-provider-boundary",
			allowCapabilityID: true,
		},
		{
			dir:               "generic_event_intelligence_boundary",
			packageName:       "leia-generic-ai-event-intelligence-boundary",
			allowCapabilityID: true,
		},
		{
			dir:               "generic_strategy_backtest_contracts",
			packageName:       "leia-generic-ai-strategy-backtest-contracts",
			allowCapabilityID: true,
		},
		{
			dir:               "generic_transcript_pipeline",
			packageName:       "leia-generic-ai-transcript-pipeline",
			allowCapabilityID: true,
		},
		{
			dir:               "generic_optional_adapter_boundary",
			packageName:       "leia-generic-ai-optional-adapter-boundary",
			allowCapabilityID: true,
		},
		{
			dir:               "generic_product_app_boundary",
			packageName:       "leia-generic-ai-product-app-boundary",
			allowCapabilityID: true,
		},
		{
			dir:               "generic_report_render_contracts",
			packageName:       "leia-generic-ai-report-render-contracts",
			allowCapabilityID: true,
		},
		{
			dir:               "generic_ui_snapshot_evaluator",
			packageName:       "leia-generic-ai-ui-snapshot-evaluator",
			allowCapabilityID: true,
		},
	} {
		t.Run(tc.dir, func(t *testing.T) {
			base := filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", tc.dir)
			var manifest map[string]json.RawMessage
			readJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)

			var packageName string
			if err := json.Unmarshal(manifest["package_name"], &packageName); err != nil {
				t.Fatalf("package_name: %v", err)
			}
			if packageName != tc.packageName {
				t.Fatalf("package_name = %q, want %q", packageName, tc.packageName)
			}

			var capabilities []string
			if err := json.Unmarshal(manifest["capabilities"], &capabilities); err != nil {
				t.Fatalf("capabilities must be an explicit string array: %v", err)
			}
			if len(capabilities) == 0 {
				t.Fatal("capabilities must not be empty")
			}

			var entrypoints map[string]string
			if err := json.Unmarshal(manifest["entrypoints"], &entrypoints); err != nil {
				t.Fatalf("entrypoints must map names to file paths only: %v", err)
			}
			for name, rel := range entrypoints {
				if rel == "" || filepath.IsAbs(rel) || strings.Contains(rel, "://") {
					t.Fatalf("entrypoint %q is not a relative file path: %q", name, rel)
				}
				if _, err := os.Stat(filepath.Join(base, filepath.FromSlash(rel))); err != nil {
					t.Fatalf("entrypoint %q path %q: %v", name, rel, err)
				}
			}

			var guarantee struct {
				Required  bool   `json:"required"`
				Statement string `json:"statement"`
			}
			if err := json.Unmarshal(manifest["no_built_in_guarantee"], &guarantee); err != nil {
				t.Fatalf("no_built_in_guarantee: %v", err)
			}
			if !guarantee.Required || !strings.Contains(guarantee.Statement, packageName) {
				t.Fatalf("no_built_in_guarantee must be required and name %q: %#v", packageName, guarantee)
			}

			for _, key := range tc.dialectKeys {
				if _, ok := manifest[key]; !ok {
					t.Fatalf("missing dialect field %q", key)
				}
			}
			if !tc.allowCapabilityID {
				if _, ok := manifest["capability_id"]; ok {
					t.Fatalf("old dialect symbol field %q must use a dialect_* name", "capability_id")
				}
			}
			for _, oldKey := range []string{"backend_shape"} {
				if _, ok := manifest[oldKey]; ok {
					t.Fatalf("old dialect symbol field %q must use a dialect_* name", oldKey)
				}
			}
		})
	}
}
