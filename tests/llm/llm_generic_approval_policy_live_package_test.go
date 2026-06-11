package leia_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

type genericApprovalPolicyManifest struct {
	SchemaVersion               int    `json:"schema_version"`
	ID                          string `json:"id"`
	PackageName                 string `json:"package_name"`
	CompanionPackageName        string `json:"companion_package_name"`
	ProviderFree                bool   `json:"provider_free"`
	LiveNetworkDefault          bool   `json:"live_network_default"`
	RealDependencyImportDefault bool   `json:"real_dependency_import_default"`
	Credentials                 struct {
		Required          []string `json:"required"`
		Optional          []string `json:"optional"`
		SecretEnvPatterns []string `json:"secret_env_patterns"`
		Policy            string   `json:"policy"`
	} `json:"credentials"`
	DefaultPolicy struct {
		Mode                       string `json:"mode"`
		LiveNetwork                bool   `json:"live_network"`
		ProviderCredentialsRequire bool   `json:"provider_credentials_required"`
		RealDependencyImports      bool   `json:"real_dependency_imports"`
		DefaultDecision            string `json:"default_decision"`
		CleanSkipWithoutDependency bool   `json:"clean_skip_without_dependency"`
		FixtureHook                string `json:"fixture_hook"`
	} `json:"default_policy"`
	Entrypoints  map[string]string `json:"entrypoints"`
	Schemas      map[string]string `json:"schemas"`
	Fixtures     map[string]string `json:"fixtures"`
	Capabilities []string          `json:"capabilities"`
	RiskLevels   []string          `json:"risk_levels"`
	TestGates    []string          `json:"test_gates"`
}

type genericApprovalPolicyRiskLevel struct {
	ID               string   `json:"id"`
	ApprovalRequired bool     `json:"approval_required"`
	DefaultOutcome   string   `json:"default_outcome"`
	Examples         []string `json:"examples"`
}

func TestGenericApprovalPolicyLivePackageManifest(t *testing.T) {
	base := genericApprovalPolicyPackageDir(t)
	manifest := loadGenericApprovalPolicyManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.ID != "generic-ai-approval-policy-live-package" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.PackageName != "generic.ai.approval.policy" || manifest.CompanionPackageName != "generic.ai.capability.gate" {
		t.Fatalf("package names = %q / %q", manifest.PackageName, manifest.CompanionPackageName)
	}
	if !manifest.ProviderFree || manifest.LiveNetworkDefault || manifest.RealDependencyImportDefault {
		t.Fatalf("provider-free defaults = provider_free:%v live_network:%v imports:%v", manifest.ProviderFree, manifest.LiveNetworkDefault, manifest.RealDependencyImportDefault)
	}
	if len(manifest.Credentials.Required) != 0 || len(manifest.Credentials.Optional) != 0 || len(manifest.Credentials.SecretEnvPatterns) != 0 {
		t.Fatalf("skeleton must not declare credentials: %#v", manifest.Credentials)
	}
	if !strings.Contains(strings.ToLower(manifest.Credentials.Policy), "no credentials") {
		t.Fatalf("credential policy should document no credentials: %q", manifest.Credentials.Policy)
	}
	if manifest.DefaultPolicy.Mode != "fixture_replay" ||
		manifest.DefaultPolicy.LiveNetwork ||
		manifest.DefaultPolicy.ProviderCredentialsRequire ||
		manifest.DefaultPolicy.RealDependencyImports ||
		manifest.DefaultPolicy.DefaultDecision != "deny" ||
		!manifest.DefaultPolicy.CleanSkipWithoutDependency ||
		manifest.DefaultPolicy.FixtureHook != "recorded_generic_approval_policy_fixture" {
		t.Fatalf("default policy must stay fixture-only, default-deny, and clean-skip safe: %#v", manifest.DefaultPolicy)
	}

	for _, key := range []string{"smoke", "approval_policy_contract", "capability_gate_contract", "fixture_index", "policy_outcomes", "approval_trace", "clean_skip"} {
		if manifest.Entrypoints[key] == "" {
			t.Fatalf("missing entrypoint %q", key)
		}
		assertGenericApprovalPolicyJSONOrLeiaFile(t, filepath.Join(base, manifest.Entrypoints[key]))
	}
	for _, key := range []string{"capability_vocabulary", "capability_gate", "approval_trace_envelope", "policy_outcome", "clean_skip"} {
		path := manifest.Schemas[key]
		if path == "" {
			t.Fatalf("missing schema %q", key)
		}
		assertGenericApprovalPolicyJSONFile(t, filepath.Join(base, path))
	}
	for _, path := range manifest.Fixtures {
		assertGenericApprovalPolicyJSONFile(t, filepath.Join(base, path))
	}

	wantRisk := []string{"critical", "high", "low", "medium"}
	gotRisk := append([]string(nil), manifest.RiskLevels...)
	sort.Strings(gotRisk)
	if !reflect.DeepEqual(gotRisk, wantRisk) {
		t.Fatalf("risk levels = %#v, want %#v", gotRisk, wantRisk)
	}
	if len(manifest.Capabilities) < 10 {
		t.Fatalf("capabilities = %d, want at least 10", len(manifest.Capabilities))
	}
	for _, capability := range manifest.Capabilities {
		if !strings.HasPrefix(capability, "generic.ai.capability.") && !strings.HasPrefix(capability, "generic.ai.approval.policy.") {
			t.Fatalf("capability %q is outside generic AI approval vocabulary", capability)
		}
	}

	joinedGates := strings.ToLower(strings.Join(manifest.TestGates, " "))
	for _, want := range []string{"provider_free", "default", "deny", "trace envelope", "clean skip", "policy outcome"} {
		if !strings.Contains(joinedGates, want) {
			t.Fatalf("test gates missing %q: %s", want, joinedGates)
		}
	}
}

func TestGenericApprovalPolicyContractsAndFixtures(t *testing.T) {
	base := genericApprovalPolicyPackageDir(t)

	var policy struct {
		PackageName                string `json:"package_name"`
		ProviderFree               bool   `json:"provider_free"`
		LiveNetwork                bool   `json:"live_network"`
		RealDependencyImports      bool   `json:"real_dependency_imports"`
		CleanSkipWithoutDependency bool   `json:"clean_skip_without_dependency"`
		DefaultPolicy              struct {
			Decision          string `json:"decision"`
			UnknownCapability string `json:"unknown_capability"`
			MissingApproval   string `json:"missing_approval"`
			DependencyAbsent  string `json:"dependency_absent"`
		} `json:"default_policy"`
		RiskLevels            []genericApprovalPolicyRiskLevel `json:"risk_levels"`
		ApprovalTraceEnvelope struct {
			Kind                 string   `json:"kind"`
			RequiredFields       []string `json:"required_fields"`
			DecisionStatusValues []string `json:"decision_status_values"`
			ResultStatusValues   []string `json:"result_status_values"`
			SecretValuesPresent  bool     `json:"secret_values_present"`
		} `json:"approval_trace_envelope"`
	}
	decodeGenericApprovalPolicyJSONFile(t, filepath.Join(base, "contracts", "generic_approval_policy_contract.json"), &policy)
	if policy.PackageName != "generic.ai.approval.policy" || !policy.ProviderFree || policy.LiveNetwork || policy.RealDependencyImports || !policy.CleanSkipWithoutDependency {
		t.Fatalf("policy contract header must stay provider-free: %#v", policy)
	}
	if policy.DefaultPolicy.Decision != "deny" || policy.DefaultPolicy.UnknownCapability != "deny" || policy.DefaultPolicy.MissingApproval != "deny" || policy.DefaultPolicy.DependencyAbsent != "skipped" {
		t.Fatalf("default-deny policy is incomplete: %#v", policy.DefaultPolicy)
	}
	assertGenericRiskLevels(t, policy.RiskLevels)
	assertStringSet(t, policy.ApprovalTraceEnvelope.DecisionStatusValues, []string{"allowed", "denied", "skipped"})
	assertStringSet(t, policy.ApprovalTraceEnvelope.ResultStatusValues, []string{"denied", "ok", "skipped"})
	if policy.ApprovalTraceEnvelope.Kind != "generic_ai_approval_trace" || policy.ApprovalTraceEnvelope.SecretValuesPresent {
		t.Fatalf("trace envelope metadata = %#v", policy.ApprovalTraceEnvelope)
	}
	for _, field := range []string{"trace_id", "fixture_key", "provider_free", "request", "policy", "decision", "result", "replay"} {
		if !containsGenericApprovalPolicyString(policy.ApprovalTraceEnvelope.RequiredFields, field) {
			t.Fatalf("trace envelope missing required field %q", field)
		}
	}

	var gate struct {
		PackageName          string `json:"package_name"`
		ProviderFree         bool   `json:"provider_free"`
		LiveNetwork          bool   `json:"live_network"`
		RealDependencyImport bool   `json:"real_dependency_imports"`
		CapabilityVocabulary []struct {
			ID               string `json:"id"`
			Capability       string `json:"capability"`
			RiskLevel        string `json:"risk_level"`
			DefaultDecision  string `json:"default_decision"`
			RequiresApproval bool   `json:"requires_approval"`
			CleanSkip        bool   `json:"clean_skip"`
			ProviderBinding  string `json:"provider_binding"`
		} `json:"capability_vocabulary"`
		GateDefaults struct {
			ExactCapabilityMatchRequired bool   `json:"exact_capability_match_required"`
			UnknownCapability            string `json:"unknown_capability"`
			MissingDependency            string `json:"missing_dependency"`
			MissingCredential            string `json:"missing_credential"`
			MissingLiveNetwork           string `json:"missing_live_network"`
			SecretValuesPresent          bool   `json:"secret_values_present"`
		} `json:"gate_defaults"`
	}
	decodeGenericApprovalPolicyJSONFile(t, filepath.Join(base, "contracts", "generic_capability_gate_contract.json"), &gate)
	if gate.PackageName != "generic.ai.capability.gate" || !gate.ProviderFree || gate.LiveNetwork || gate.RealDependencyImport {
		t.Fatalf("capability gate header must stay provider-free: %#v", gate)
	}
	if !gate.GateDefaults.ExactCapabilityMatchRequired || gate.GateDefaults.UnknownCapability != "deny" ||
		gate.GateDefaults.MissingDependency != "skipped" || gate.GateDefaults.MissingCredential != "skipped" ||
		gate.GateDefaults.MissingLiveNetwork != "skipped" || gate.GateDefaults.SecretValuesPresent {
		t.Fatalf("capability gate defaults incomplete: %#v", gate.GateDefaults)
	}
	if len(gate.CapabilityVocabulary) != 7 {
		t.Fatalf("capability vocabulary entries = %d, want 7", len(gate.CapabilityVocabulary))
	}
	seen := map[string]bool{}
	highRiskDenyCount := 0
	for _, entry := range gate.CapabilityVocabulary {
		if entry.ID == "" || !strings.HasPrefix(entry.Capability, "generic.ai.capability.") {
			t.Fatalf("capability entry incomplete: %#v", entry)
		}
		if seen[entry.Capability] {
			t.Fatalf("duplicate capability %q", entry.Capability)
		}
		seen[entry.Capability] = true
		if entry.RiskLevel != "low" && entry.DefaultDecision != "deny" {
			t.Fatalf("%s non-low risk must deny by default: %#v", entry.ID, entry)
		}
		if entry.RiskLevel != "low" && (!entry.RequiresApproval || !entry.CleanSkip || entry.ProviderBinding != "external") {
			t.Fatalf("%s high-risk gate must require approval and clean-skip external bindings: %#v", entry.ID, entry)
		}
		if entry.DefaultDecision == "deny" {
			highRiskDenyCount++
		}
	}
	if highRiskDenyCount != 6 {
		t.Fatalf("default deny count = %d, want 6", highRiskDenyCount)
	}

	assertGenericApprovalPolicyFixtureIndex(t, base)
	assertGenericApprovalPolicyOutcomes(t, base)
	assertGenericApprovalPolicyTrace(t, base)
	assertGenericApprovalPolicyCleanSkip(t, base)
}

func TestGenericApprovalPolicySmokeLeia(t *testing.T) {
	base := genericApprovalPolicyPackageDir(t)
	data, err := os.ReadFile(filepath.Join(base, "main.leia"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, pattern := range []string{
		`(?m)^\s*import\s+`,
		`(?m)^\s*use\s+`,
		`(?m)^\s*load\s*\(`,
		`(?m)^\s*require\s*\(`,
	} {
		if regexp.MustCompile(pattern).FindString(source) != "" {
			t.Fatalf("main.leia contains live dependency loader matching %q", pattern)
		}
	}

	for _, mode := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			vm := leia.New(append([]leia.Option{leia.WithLibs(leia.LibString)}, mode.opts...)...)
			if err := vm.ExecFile(filepath.Join(base, "main.leia")); err != nil {
				t.Fatalf("ExecFile: %v", err)
			}
			got, _ := vm.Get("generic_approval_policy_summary")
			want := "generic_approval_policy capabilities=7 default_deny=6 clean_skip=6 provider_free=true live_network=false imports=false"
			if got != want {
				t.Fatalf("summary = %#v, want %#v", got, want)
			}
		})
	}
}

func assertGenericApprovalPolicyFixtureIndex(t *testing.T, base string) {
	t.Helper()
	var index struct {
		ProviderFree          bool `json:"provider_free"`
		LiveNetwork           bool `json:"live_network"`
		RealDependencyImports bool `json:"real_dependency_imports"`
		Fixtures              []struct {
			FixtureKey string         `json:"fixture_key"`
			Capability string         `json:"capability"`
			Path       string         `json:"path"`
			Schema     string         `json:"schema"`
			Metadata   map[string]any `json:"metadata"`
		} `json:"fixtures"`
	}
	decodeGenericApprovalPolicyJSONFile(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"), &index)
	if !index.ProviderFree || index.LiveNetwork || index.RealDependencyImports || len(index.Fixtures) != 3 {
		t.Fatalf("fixture index header/count = %#v", index)
	}
	for _, fixture := range index.Fixtures {
		if fixture.FixtureKey == "" || !strings.HasPrefix(fixture.Capability, "generic.ai.approval.policy.") {
			t.Fatalf("fixture metadata incomplete: %#v", fixture)
		}
		assertGenericApprovalPolicyJSONFile(t, filepath.Join(base, fixture.Path))
		assertGenericApprovalPolicyJSONFile(t, filepath.Join(base, fixture.Schema))
		if fixture.Metadata["replay_ready"] != true {
			t.Fatalf("%s replay_ready = %#v", fixture.FixtureKey, fixture.Metadata["replay_ready"])
		}
	}
}

func assertGenericApprovalPolicyOutcomes(t *testing.T, base string) {
	t.Helper()
	var fixture struct {
		ProviderFree          bool   `json:"provider_free"`
		LiveNetwork           bool   `json:"live_network"`
		RealDependencyImports bool   `json:"real_dependency_imports"`
		DefaultDecision       string `json:"default_decision"`
		Outcomes              []struct {
			ID               string `json:"id"`
			Capability       string `json:"capability"`
			RiskLevel        string `json:"risk_level"`
			RequiresApproval bool   `json:"requires_approval"`
			Grant            string `json:"grant"`
			Decision         string `json:"decision"`
			ResultStatus     string `json:"result_status"`
			CleanSkip        bool   `json:"clean_skip"`
		} `json:"outcomes"`
	}
	decodeGenericApprovalPolicyJSONFile(t, filepath.Join(base, "fixtures", "policy_outcome_matrix_fixture.json"), &fixture)
	if !fixture.ProviderFree || fixture.LiveNetwork || fixture.RealDependencyImports || fixture.DefaultDecision != "deny" {
		t.Fatalf("outcome fixture header must stay provider-free default-deny: %#v", fixture)
	}
	decisions := map[string]bool{}
	for _, outcome := range fixture.Outcomes {
		if outcome.ID == "" || !strings.HasPrefix(outcome.Capability, "generic.ai.capability.") {
			t.Fatalf("outcome incomplete: %#v", outcome)
		}
		decisions[outcome.Decision] = true
		if outcome.RiskLevel != "low" && outcome.Decision == "allowed" {
			t.Fatalf("non-low risk outcome must not allow by default: %#v", outcome)
		}
		if outcome.Decision == "skipped" && (!outcome.CleanSkip || outcome.ResultStatus != "skipped") {
			t.Fatalf("skip outcome must be clean: %#v", outcome)
		}
		if outcome.Decision == "denied" && outcome.ResultStatus != "denied" {
			t.Fatalf("deny outcome result mismatch: %#v", outcome)
		}
	}
	for _, want := range []string{"allowed", "denied", "skipped"} {
		if !decisions[want] {
			t.Fatalf("policy outcome fixture missing decision %q", want)
		}
	}
}

func assertGenericApprovalPolicyTrace(t *testing.T, base string) {
	t.Helper()
	var trace struct {
		Kind                  string `json:"kind"`
		ProviderFree          bool   `json:"provider_free"`
		LiveNetwork           bool   `json:"live_network"`
		RealDependencyImports bool   `json:"real_dependency_imports"`
		Request               struct {
			Capability string `json:"capability"`
			RiskLevel  string `json:"risk_level"`
		} `json:"request"`
		Policy struct {
			Package                      string   `json:"package"`
			DefaultDecision              string   `json:"default_decision"`
			ExactCapabilityMatchRequired bool     `json:"exact_capability_match_required"`
			AllowedCapabilities          []string `json:"allowed_capabilities"`
		} `json:"policy"`
		Decision struct {
			Status           string  `json:"status"`
			Reason           string  `json:"reason"`
			ApprovalRequired bool    `json:"approval_required"`
			ApprovalID       *string `json:"approval_id"`
		} `json:"decision"`
		Result struct {
			Status              string `json:"status"`
			Executed            bool   `json:"executed"`
			SideEffects         bool   `json:"side_effects"`
			SecretValuesPresent bool   `json:"secret_values_present"`
		} `json:"result"`
		Replay struct {
			Mode                string `json:"mode"`
			Deterministic       bool   `json:"deterministic"`
			CreatedFromProvider bool   `json:"created_from_provider"`
		} `json:"replay"`
	}
	decodeGenericApprovalPolicyJSONFile(t, filepath.Join(base, "fixtures", "approval_trace_envelope_fixture.json"), &trace)
	if trace.Kind != "generic_ai_approval_trace" || !trace.ProviderFree || trace.LiveNetwork || trace.RealDependencyImports {
		t.Fatalf("trace header must stay provider-free: %#v", trace)
	}
	if trace.Request.Capability != "generic.ai.capability.trading.order_submit" || trace.Request.RiskLevel != "critical" {
		t.Fatalf("trace request mismatch: %#v", trace.Request)
	}
	if trace.Policy.Package != "generic.ai.approval.policy" || trace.Policy.DefaultDecision != "deny" || !trace.Policy.ExactCapabilityMatchRequired || len(trace.Policy.AllowedCapabilities) != 0 {
		t.Fatalf("trace policy must be default-deny with exact grants: %#v", trace.Policy)
	}
	if trace.Decision.Status != "denied" || !trace.Decision.ApprovalRequired || trace.Decision.ApprovalID != nil || trace.Decision.Reason == "" {
		t.Fatalf("trace decision mismatch: %#v", trace.Decision)
	}
	if trace.Result.Status != "denied" || trace.Result.Executed || trace.Result.SideEffects || trace.Result.SecretValuesPresent {
		t.Fatalf("trace result must show no side effects or secrets: %#v", trace.Result)
	}
	if trace.Replay.Mode != "fixture_replay" || !trace.Replay.Deterministic || trace.Replay.CreatedFromProvider {
		t.Fatalf("trace replay metadata mismatch: %#v", trace.Replay)
	}
}

func assertGenericApprovalPolicyCleanSkip(t *testing.T, base string) {
	t.Helper()
	var skip struct {
		ProviderFree          bool   `json:"provider_free"`
		LiveNetwork           bool   `json:"live_network"`
		RealDependencyImports bool   `json:"real_dependency_imports"`
		RequestedCapability   string `json:"requested_capability"`
		Status                string `json:"status"`
		CleanSkip             bool   `json:"clean_skip"`
		DependencyImported    bool   `json:"dependency_imported"`
		CredentialsRequired   bool   `json:"credentials_required"`
		SecretValuesPresent   bool   `json:"secret_values_present"`
		ResultEnvelope        struct {
			Status     string `json:"status"`
			Capability string `json:"capability"`
			Metadata   struct {
				ProviderFree          bool `json:"provider_free"`
				CleanSkip             bool `json:"clean_skip"`
				LiveNetwork           bool `json:"live_network"`
				RealDependencyImports bool `json:"real_dependency_imports"`
			} `json:"metadata"`
		} `json:"result_envelope"`
	}
	decodeGenericApprovalPolicyJSONFile(t, filepath.Join(base, "fixtures", "clean_skip_fixture.json"), &skip)
	if !skip.ProviderFree || skip.LiveNetwork || skip.RealDependencyImports || skip.RequestedCapability != "generic.ai.capability.network.http" {
		t.Fatalf("clean skip header mismatch: %#v", skip)
	}
	if skip.Status != "skipped" || !skip.CleanSkip || skip.DependencyImported || skip.CredentialsRequired || skip.SecretValuesPresent {
		t.Fatalf("clean skip metadata incomplete: %#v", skip)
	}
	if skip.ResultEnvelope.Status != "skipped" || skip.ResultEnvelope.Capability != skip.RequestedCapability ||
		!skip.ResultEnvelope.Metadata.ProviderFree || !skip.ResultEnvelope.Metadata.CleanSkip ||
		skip.ResultEnvelope.Metadata.LiveNetwork || skip.ResultEnvelope.Metadata.RealDependencyImports {
		t.Fatalf("clean skip result envelope mismatch: %#v", skip.ResultEnvelope)
	}
}

func assertGenericRiskLevels(t *testing.T, levels []genericApprovalPolicyRiskLevel) {
	t.Helper()
	seen := map[string]bool{}
	for _, level := range levels {
		seen[level.ID] = true
		if level.ID == "low" {
			if level.ApprovalRequired || level.DefaultOutcome != "allow_fixture" {
				t.Fatalf("low risk policy mismatch: %#v", level)
			}
			continue
		}
		if !level.ApprovalRequired || level.DefaultOutcome != "deny" || len(level.Examples) == 0 {
			t.Fatalf("non-low risk policy mismatch: %#v", level)
		}
	}
	for _, want := range []string{"low", "medium", "high", "critical"} {
		if !seen[want] {
			t.Fatalf("missing risk level %q", want)
		}
	}
}

func genericApprovalPolicyPackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_approval_policy")
}

func loadGenericApprovalPolicyManifest(t *testing.T, base string) genericApprovalPolicyManifest {
	t.Helper()
	var manifest genericApprovalPolicyManifest
	decodeGenericApprovalPolicyJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	return manifest
}

func assertGenericApprovalPolicyJSONOrLeiaFile(t *testing.T, path string) {
	t.Helper()
	if filepath.Ext(path) == ".leia" {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
		return
	}
	assertGenericApprovalPolicyJSONFile(t, path)
}

func assertGenericApprovalPolicyJSONFile(t *testing.T, path string) {
	t.Helper()
	var value any
	decodeGenericApprovalPolicyJSONFile(t, path, &value)
}

func decodeGenericApprovalPolicyJSONFile(t *testing.T, path string, value any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func assertStringSet(t *testing.T, got, want []string) {
	t.Helper()
	gotCopy := append([]string(nil), got...)
	wantCopy := append([]string(nil), want...)
	sort.Strings(gotCopy)
	sort.Strings(wantCopy)
	if !reflect.DeepEqual(gotCopy, wantCopy) {
		t.Fatalf("strings = %#v, want %#v", gotCopy, wantCopy)
	}
}

func containsGenericApprovalPolicyString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
