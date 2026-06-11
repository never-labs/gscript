package leia_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

type finrobotPolicyParityLedger struct {
	SchemaVersion     int    `json:"schema_version"`
	ID                string `json:"id"`
	Scope             string `json:"scope"`
	NotFinRobotSyntax bool   `json:"not_finrobot_syntax"`
	Target            struct {
		BaselineBranch              string `json:"baseline_branch"`
		TranslationRoot             string `json:"translation_root"`
		LedgerDirectory             string `json:"ledger_directory"`
		PolicyExample               string `json:"policy_example"`
		ProviderFreeDefault         bool   `json:"provider_free_default"`
		LiveNetworkDefault          bool   `json:"live_network_default"`
		RealCredentialsDefault      bool   `json:"real_credentials_default"`
		RealDependencyImportDefault bool   `json:"real_dependency_import_default"`
		RuntimeExecutionDefault     bool   `json:"runtime_execution_default"`
		FileWriteDefault            bool   `json:"file_write_default"`
		PublicationDefault          bool   `json:"publication_default"`
		TradingDefault              bool   `json:"trading_default"`
		ModelCallDefault            bool   `json:"model_call_default"`
	} `json:"target"`
	GlobalGates      []string `json:"global_gates"`
	StatusValues     []string `json:"status_values"`
	ActionValues     []string `json:"action_values"`
	PolicyDimensions []struct {
		ID                    string `json:"id"`
		Capability            string `json:"capability"`
		PolicyClass           string `json:"policy_class"`
		SourceConcept         string `json:"source_concept"`
		DefaultStatus         string `json:"default_status"`
		CleanSkipPrerequisite string `json:"clean_skip_prerequisite"`
	} `json:"policy_dimensions"`
	CaseSchema     string                     `json:"case_schema"`
	OutcomeFixture string                     `json:"outcome_fixture"`
	Matrix         []finrobotPolicyParityCase `json:"matrix"`
	NegativeGates  struct {
		ForbiddenImports       []string `json:"forbidden_imports"`
		ForbiddenLiveBehaviors []string `json:"forbidden_live_behaviors"`
		ForbiddenClaims        []string `json:"forbidden_claims"`
	} `json:"negative_gates"`
}

type finrobotPolicyParityCase struct {
	ID               string `json:"id"`
	SourceConcept    string `json:"source_concept"`
	Dimension        string `json:"dimension"`
	Capability       string `json:"capability"`
	PolicyClass      string `json:"policy_class"`
	Action           string `json:"action"`
	ExpectedStatus   string `json:"expected_status"`
	RequiresApproval bool   `json:"requires_approval"`
	DefaultEnabled   bool   `json:"default_enabled"`
	CleanSkip        bool   `json:"clean_skip"`
	Fixture          string `json:"fixture"`
}

func TestFinRobotPolicyParityLedgerHeaderAndDefaults(t *testing.T) {
	base := finrobotPolicyParityDir(t)
	ledger := loadFinRobotPolicyParityLedger(t, base)

	if ledger.SchemaVersion != 1 || ledger.ID != "finrobot-approval-capability-policy-parity-ledger" {
		t.Fatalf("ledger header = schema %d id %q", ledger.SchemaVersion, ledger.ID)
	}
	if ledger.Scope != "approval_capability_policy" || !ledger.NotFinRobotSyntax {
		t.Fatalf("ledger must describe generic policy parity, not FinRobot syntax: scope=%q flag=%v", ledger.Scope, ledger.NotFinRobotSyntax)
	}
	if ledger.Target.BaselineBranch != "origin/codex/ai-dialect-polish" ||
		ledger.Target.TranslationRoot != "examples/ai/finrobot_translation" ||
		ledger.Target.LedgerDirectory != "examples/ai/finrobot_translation/policy_parity" ||
		ledger.Target.PolicyExample != "examples/ai/finrobot_translation/compliance_policy.leia" {
		t.Fatalf("target identity mismatch: %#v", ledger.Target)
	}
	if !ledger.Target.ProviderFreeDefault ||
		ledger.Target.LiveNetworkDefault ||
		ledger.Target.RealCredentialsDefault ||
		ledger.Target.RealDependencyImportDefault ||
		ledger.Target.RuntimeExecutionDefault ||
		ledger.Target.FileWriteDefault ||
		ledger.Target.PublicationDefault ||
		ledger.Target.TradingDefault ||
		ledger.Target.ModelCallDefault {
		t.Fatalf("policy parity defaults must be provider-free with all live side effects disabled: %#v", ledger.Target)
	}
	for _, rel := range []string{ledger.CaseSchema, ledger.OutcomeFixture} {
		if _, err := os.Stat(filepath.Join(base, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("referenced file %s: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(repoRoot(t), filepath.FromSlash(ledger.Target.PolicyExample))); err != nil {
		t.Fatalf("policy example %s: %v", ledger.Target.PolicyExample, err)
	}

	joinedGates := strings.ToLower(strings.Join(ledger.GlobalGates, " "))
	for _, want := range []string{"ledger driven", "deny_high_risk", "approved cases", "clean-skip", "provider-free"} {
		if !strings.Contains(joinedGates, want) {
			t.Fatalf("global gates missing %q: %s", want, joinedGates)
		}
	}
}

func TestFinRobotPolicyParityMatrixCoversEveryCapabilityAndOutcome(t *testing.T) {
	base := finrobotPolicyParityDir(t)
	ledger := loadFinRobotPolicyParityLedger(t, base)
	statusValues := finrobotPolicyStringSet(ledger.StatusValues)
	actionValues := finrobotPolicyStringSet(ledger.ActionValues)

	wantDimensions := map[string]string{
		"external_network":   "network.http",
		"credentials":        "credential.read",
		"code_execution":     "generated-code.execute",
		"file_write":         "filesystem.write",
		"report_publication": "publish.web",
		"trading_action":     "trading.order.submit",
		"provider_live_gate": "provider.live",
		"model_call_gate":    "llm.model.call.live",
	}
	dimensions := map[string]struct {
		capability    string
		policyClass   string
		sourceConcept string
		defaultStatus string
	}{}
	for _, dimension := range ledger.PolicyDimensions {
		if dimension.ID == "" || dimension.Capability == "" || dimension.PolicyClass == "" ||
			dimension.SourceConcept == "" || dimension.CleanSkipPrerequisite == "" {
			t.Fatalf("incomplete policy dimension: %#v", dimension)
		}
		if !statusValues[dimension.DefaultStatus] {
			t.Fatalf("%s unknown default status %q", dimension.ID, dimension.DefaultStatus)
		}
		dimensions[dimension.ID] = struct {
			capability    string
			policyClass   string
			sourceConcept string
			defaultStatus string
		}{dimension.Capability, dimension.PolicyClass, dimension.SourceConcept, dimension.DefaultStatus}
	}
	for want, capability := range wantDimensions {
		got, ok := dimensions[want]
		if !ok {
			t.Fatalf("policy dimension %q missing", want)
		}
		if got.capability != capability {
			t.Fatalf("dimension %s capability = %q, want %q", want, got.capability, capability)
		}
	}

	outcomes := loadFinRobotPolicyOutcomeStatuses(t, filepath.Join(base, filepath.FromSlash(ledger.OutcomeFixture)))
	matrix := map[string]map[string]finrobotPolicyParityCase{}
	caseIDs := map[string]bool{}
	for _, tc := range ledger.Matrix {
		if tc.ID == "" || tc.Dimension == "" || tc.Capability == "" || tc.PolicyClass == "" || tc.Fixture == "" {
			t.Fatalf("incomplete matrix case: %#v", tc)
		}
		if caseIDs[tc.ID] {
			t.Fatalf("duplicate matrix case id %q", tc.ID)
		}
		caseIDs[tc.ID] = true
		if !actionValues[tc.Action] || !statusValues[tc.ExpectedStatus] {
			t.Fatalf("%s has unknown action/status: %#v", tc.ID, tc)
		}
		dimension, ok := dimensions[tc.Dimension]
		if !ok {
			t.Fatalf("%s references unknown dimension %q", tc.ID, tc.Dimension)
		}
		if tc.Capability != dimension.capability || tc.PolicyClass != dimension.policyClass || tc.SourceConcept != dimension.sourceConcept {
			t.Fatalf("%s does not match dimension metadata: case=%#v dimension=%#v", tc.ID, tc, dimension)
		}
		if tc.Action == "approve" && (!tc.RequiresApproval || tc.DefaultEnabled || tc.CleanSkip || tc.ExpectedStatus != "approved") {
			t.Fatalf("%s approve semantics are wrong: %#v", tc.ID, tc)
		}
		if tc.Action == "deny" && (tc.RequiresApproval || tc.DefaultEnabled || tc.CleanSkip || tc.ExpectedStatus != "denied") {
			t.Fatalf("%s deny semantics are wrong: %#v", tc.ID, tc)
		}
		if tc.Action == "clean_skip" && (tc.RequiresApproval || tc.DefaultEnabled || !tc.CleanSkip || tc.ExpectedStatus != "clean_skip") {
			t.Fatalf("%s clean-skip semantics are wrong: %#v", tc.ID, tc)
		}
		if statusFromFixture(tc.Fixture) != tc.ExpectedStatus || !outcomes[tc.ExpectedStatus] {
			t.Fatalf("%s fixture/status mismatch: fixture=%q status=%q outcomes=%#v", tc.ID, tc.Fixture, tc.ExpectedStatus, outcomes)
		}
		if matrix[tc.Dimension] == nil {
			matrix[tc.Dimension] = map[string]finrobotPolicyParityCase{}
		}
		matrix[tc.Dimension][tc.Action] = tc
	}
	for dimension := range wantDimensions {
		cases := matrix[dimension]
		for _, action := range []string{"deny", "approve", "clean_skip"} {
			if cases[action].ID == "" {
				t.Fatalf("dimension %s missing action %s in ledger-driven matrix", dimension, action)
			}
		}
	}
	if len(ledger.Matrix) != len(wantDimensions)*len(ledger.ActionValues) {
		t.Fatalf("matrix case count = %d, want %d", len(ledger.Matrix), len(wantDimensions)*len(ledger.ActionValues))
	}
}

func TestFinRobotPolicyParityFilesStayProviderFree(t *testing.T) {
	base := finrobotPolicyParityDir(t)
	ledger := loadFinRobotPolicyParityLedger(t, base)
	files := []string{
		filepath.Join(base, "README.md"),
		filepath.Join(base, "ledger.json"),
		filepath.Join(base, filepath.FromSlash(ledger.CaseSchema)),
		filepath.Join(base, filepath.FromSlash(ledger.OutcomeFixture)),
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(data)
		for _, blocked := range ledger.NegativeGates.ForbiddenImports {
			if strings.Contains(text, "import "+blocked) ||
				strings.Contains(text, `require("`+blocked+`"`) ||
				strings.Contains(text, "from "+blocked+" import") {
				t.Fatalf("%s appears to import forbidden runtime dependency %q", path, blocked)
			}
		}
		for _, networkMarker := range []string{"https://", "http://"} {
			if strings.Contains(text, networkMarker) {
				t.Fatalf("%s contains live network locator %q", path, networkMarker)
			}
		}
	}
}

func TestFinRobotPolicyParityReferencedCompliancePolicyRuns(t *testing.T) {
	ledger := loadFinRobotPolicyParityLedger(t, finrobotPolicyParityDir(t))
	src, err := os.ReadFile(filepath.Join(repoRoot(t), filepath.FromSlash(ledger.Target.PolicyExample)))
	if err != nil {
		t.Fatalf("read policy example: %v", err)
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
				t.Fatalf("Exec %s: %v", ledger.Target.PolicyExample, err)
			}
			for name, want := range map[string]any{
				"ok": true,
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

func finrobotPolicyParityDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "policy_parity")
}

func loadFinRobotPolicyParityLedger(t *testing.T, base string) finrobotPolicyParityLedger {
	t.Helper()
	var ledger finrobotPolicyParityLedger
	decodeFinRobotPolicyParityJSONFile(t, filepath.Join(base, "ledger.json"), &ledger)
	return ledger
}

func loadFinRobotPolicyOutcomeStatuses(t *testing.T, path string) map[string]bool {
	t.Helper()
	var fixture struct {
		SchemaVersion int  `json:"schema_version"`
		ProviderFree  bool `json:"provider_free"`
		Outcomes      []struct {
			Status             string `json:"status"`
			OK                 bool   `json:"ok"`
			Decision           string `json:"decision"`
			TraceKind          string `json:"trace_kind"`
			PolicyDefault      string `json:"policy_default"`
			SideEffectExecuted bool   `json:"side_effect_executed"`
		} `json:"outcomes"`
	}
	decodeFinRobotPolicyParityJSONFile(t, path, &fixture)
	if fixture.SchemaVersion != 1 || !fixture.ProviderFree {
		t.Fatalf("outcome fixture header invalid: %#v", fixture)
	}
	statuses := map[string]bool{}
	for _, outcome := range fixture.Outcomes {
		if outcome.Status == "" || outcome.Decision == "" || outcome.TraceKind == "" || outcome.PolicyDefault != "deny_high_risk" {
			t.Fatalf("incomplete policy outcome: %#v", outcome)
		}
		if outcome.SideEffectExecuted {
			t.Fatalf("policy parity fixture must not execute side effects: %#v", outcome)
		}
		if outcome.Status == "denied" && outcome.OK {
			t.Fatalf("denied outcome must not be ok: %#v", outcome)
		}
		if outcome.Status != "denied" && !outcome.OK {
			t.Fatalf("%s outcome should be an ok replay/skip envelope: %#v", outcome.Status, outcome)
		}
		statuses[outcome.Status] = true
	}
	return statuses
}

func decodeFinRobotPolicyParityJSONFile(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func finrobotPolicyStringSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		out[value] = true
	}
	return out
}

func statusFromFixture(ref string) string {
	_, status, ok := strings.Cut(ref, "#")
	if !ok {
		return ""
	}
	return status
}
