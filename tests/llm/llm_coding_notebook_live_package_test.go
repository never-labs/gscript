package leia_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type codingNotebookLiveManifest struct {
	SchemaVersion               int      `json:"schema_version"`
	ID                          string   `json:"id"`
	PackageName                 string   `json:"package_name"`
	ProviderFree                bool     `json:"provider_free"`
	LiveNetworkDefault          bool     `json:"live_network_default"`
	RealDependencyImportDefault bool     `json:"real_dependency_import_default"`
	SourceModules               []string `json:"source_modules"`
	Credentials                 struct {
		Required          []string `json:"required"`
		Optional          []string `json:"optional"`
		SecretEnvPatterns []string `json:"secret_env_patterns"`
		Policy            string   `json:"policy"`
	} `json:"credentials"`
	DefaultPolicy struct {
		Mode                       string `json:"mode"`
		LiveNetwork                bool   `json:"live_network"`
		RealDependencyImports      bool   `json:"real_dependency_imports"`
		SandboxExecutionDefault    string `json:"sandbox_execution_default"`
		FileWriteDefault           string `json:"file_write_default"`
		ImageDisplayDefault        string `json:"image_display_default"`
		CleanSkipWithoutDependency bool   `json:"clean_skip_without_dependency"`
		FixtureHook                string `json:"fixture_hook"`
	} `json:"default_policy"`
	Entrypoints        map[string]string                  `json:"entrypoints"`
	Schemas            map[string]string                  `json:"schemas"`
	Fixtures           map[string]string                  `json:"fixtures"`
	Capabilities       []string                           `json:"capabilities"`
	CapabilityMetadata []codingNotebookCapabilityMetadata `json:"capability_metadata"`
	TestGates          []string                           `json:"test_gates"`
	NoBuiltInGuarantee struct {
		Required bool `json:"required"`
	} `json:"no_built_in_guarantee"`
}

type codingNotebookCapabilityMetadata struct {
	ID               string `json:"id"`
	Capability       string `json:"capability"`
	Default          string `json:"default"`
	Schema           string `json:"schema"`
	ApprovalRequired bool   `json:"approval_required"`
}

func TestFinRobotCodingNotebookLivePackageManifest(t *testing.T) {
	base := codingNotebookLivePackageDir(t)
	manifest := loadCodingNotebookLiveManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.ID != "finrobot-coding-notebook-live-package" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.PackageName != "leia-finrobot-coding-notebook" {
		t.Fatalf("package name = %q", manifest.PackageName)
	}
	if !manifest.ProviderFree || manifest.LiveNetworkDefault || manifest.RealDependencyImportDefault {
		t.Fatalf("provider-free defaults = provider_free:%v live_network:%v imports:%v", manifest.ProviderFree, manifest.LiveNetworkDefault, manifest.RealDependencyImportDefault)
	}
	if len(manifest.Credentials.Required) != 0 || len(manifest.Credentials.Optional) != 0 || len(manifest.Credentials.SecretEnvPatterns) != 0 {
		t.Fatalf("skeleton must not declare credentials: %#v", manifest.Credentials)
	}
	if !strings.Contains(manifest.Credentials.Policy, "notebook kernels") || !strings.Contains(manifest.Credentials.Policy, "execution sandboxes") {
		t.Fatalf("credential policy should name future external boundaries: %q", manifest.Credentials.Policy)
	}
	if manifest.DefaultPolicy.Mode != "fixture_replay" ||
		manifest.DefaultPolicy.LiveNetwork ||
		manifest.DefaultPolicy.RealDependencyImports ||
		manifest.DefaultPolicy.SandboxExecutionDefault != "deny" ||
		manifest.DefaultPolicy.FileWriteDefault != "deny" ||
		manifest.DefaultPolicy.ImageDisplayDefault != "fixture_only" ||
		!manifest.DefaultPolicy.CleanSkipWithoutDependency ||
		manifest.DefaultPolicy.FixtureHook != "recorded_coding_notebook_fixture" {
		t.Fatalf("default policy must stay fixture-only and deny unsafe operations: %#v", manifest.DefaultPolicy)
	}

	wantSources := []string{
		"finrobot/functional/coding.py",
		"examples/ai/finrobot_translation/generated_code_tooling.leia",
	}
	if !reflect.DeepEqual(manifest.SourceModules, wantSources) {
		t.Fatalf("source modules = %#v, want %#v", manifest.SourceModules, wantSources)
	}

	for _, key := range []string{"coding_notebook_contract", "sandbox_policy_gates", "fixture_index", "denied_command_fixture", "deterministic_replay_fixture"} {
		if manifest.Entrypoints[key] == "" {
			t.Fatalf("missing entrypoint %q", key)
		}
		assertCodingNotebookJSONFile(t, filepath.Join(base, manifest.Entrypoints[key]))
	}
	for _, key := range []string{"sandbox_command", "file_image_artifact", "notebook_display", "capability_metadata"} {
		if manifest.Schemas[key] == "" {
			t.Fatalf("missing schema %q", key)
		}
		assertCodingNotebookJSONFile(t, filepath.Join(base, manifest.Schemas[key]))
	}
	for _, key := range []string{"index", "denied_command", "deterministic_replay"} {
		if manifest.Fixtures[key] == "" {
			t.Fatalf("missing fixture %q", key)
		}
		assertCodingNotebookJSONFile(t, filepath.Join(base, manifest.Fixtures[key]))
	}

	wantCapabilities := []string{
		"coding.notebook.artifact.publish",
		"coding.notebook.command.deny",
		"coding.notebook.command.execute",
		"coding.notebook.file.read",
		"coding.notebook.file.write",
		"coding.notebook.image.display",
		"coding.notebook.metadata.capability",
		"coding.notebook.replay.deterministic",
		"coding.notebook.stderr.capture",
		"coding.notebook.stdout.capture",
	}
	gotCapabilities := append([]string(nil), manifest.Capabilities...)
	sort.Strings(gotCapabilities)
	if !reflect.DeepEqual(gotCapabilities, wantCapabilities) {
		t.Fatalf("capabilities = %#v, want %#v", gotCapabilities, wantCapabilities)
	}
	if !manifest.NoBuiltInGuarantee.Required {
		t.Fatal("coding notebook package must declare no built-in guarantee")
	}

	joinedGates := strings.ToLower(strings.Join(manifest.TestGates, " "))
	for _, want := range []string{"sandbox", "file", "image", "denied command", "stdout", "stderr", "deterministic replay", "capability metadata"} {
		if !strings.Contains(joinedGates, want) {
			t.Fatalf("test gates missing %q: %s", want, joinedGates)
		}
	}
}

func TestFinRobotCodingNotebookPolicyFixturesAndArtifactSchema(t *testing.T) {
	base := codingNotebookLivePackageDir(t)

	var policy struct {
		ProviderFree          bool   `json:"provider_free"`
		LiveNetwork           bool   `json:"live_network"`
		RealDependencyImports bool   `json:"real_dependency_imports"`
		DefaultDecision       string `json:"default_decision"`
		ApprovalGates         []struct {
			ID       string `json:"id"`
			Decision string `json:"decision"`
		} `json:"approval_gates"`
		CaptureContract struct {
			StdoutRequired               bool `json:"stdout_required"`
			StderrRequired               bool `json:"stderr_required"`
			ExitCodeRequired             bool `json:"exit_code_required"`
			ApprovalDecisionRequired     bool `json:"approval_decision_required"`
			DeniedCommandsMustHaveReason bool `json:"denied_commands_must_have_reason"`
		} `json:"capture_contract"`
	}
	decodeCodingNotebookJSONFile(t, filepath.Join(base, "contracts", "sandbox_policy_gates.json"), &policy)
	if !policy.ProviderFree || policy.LiveNetwork || policy.RealDependencyImports || policy.DefaultDecision != "deny" {
		t.Fatalf("policy header/default decision = %#v", policy)
	}
	if !policy.CaptureContract.StdoutRequired || !policy.CaptureContract.StderrRequired || !policy.CaptureContract.ExitCodeRequired ||
		!policy.CaptureContract.ApprovalDecisionRequired || !policy.CaptureContract.DeniedCommandsMustHaveReason {
		t.Fatalf("capture contract incomplete: %#v", policy.CaptureContract)
	}
	decisions := map[string]string{}
	for _, gate := range policy.ApprovalGates {
		decisions[gate.ID] = gate.Decision
	}
	if decisions["deny_live_notebook_kernel"] != "deny" || decisions["deny_unapproved_file_write"] != "deny" || decisions["allow_fixture_image_display"] != "allow_fixture" {
		t.Fatalf("approval gate decisions = %#v", decisions)
	}

	var denied struct {
		ProviderFree bool   `json:"provider_free"`
		LiveNetwork  bool   `json:"live_network"`
		ReplayKey    string `json:"replay_key"`
		Capability   string `json:"capability"`
		Approval     struct {
			Decision string `json:"decision"`
			Reason   string `json:"reason"`
		} `json:"approval"`
		Result struct {
			ExitCode int    `json:"exit_code"`
			Stdout   string `json:"stdout"`
			Stderr   string `json:"stderr"`
		} `json:"result"`
		Deterministic bool `json:"deterministic"`
	}
	decodeCodingNotebookJSONFile(t, filepath.Join(base, "fixtures", "denied_command_fixture.json"), &denied)
	if !denied.ProviderFree || denied.LiveNetwork || denied.ReplayKey != "command:deny:notebook-kernel" ||
		denied.Capability != "coding.notebook.command.execute" || denied.Approval.Decision != "deny" ||
		denied.Approval.Reason == "" || denied.Result.ExitCode == 0 || denied.Result.Stdout != "" ||
		!strings.Contains(denied.Result.Stderr, "denied") || !denied.Deterministic {
		t.Fatalf("denied command fixture = %#v", denied)
	}

	var artifactSchema struct {
		Required   []string `json:"required"`
		Properties struct {
			MediaType struct {
				Enum []string `json:"enum"`
			} `json:"media_type"`
			Provenance struct {
				Required []string `json:"required"`
			} `json:"provenance"`
		} `json:"properties"`
	}
	decodeCodingNotebookJSONFile(t, filepath.Join(base, "schemas", "file_image_artifact.schema.json"), &artifactSchema)
	for _, want := range []string{"artifact_id", "replay_key", "path", "media_type", "sha256", "bytes", "provenance"} {
		if !contains(artifactSchema.Required, want) {
			t.Fatalf("artifact schema missing required field %q: %#v", want, artifactSchema.Required)
		}
	}
	for _, want := range []string{"application/vnd.jupyter", "text/x-python", "image/png"} {
		if !contains(artifactSchema.Properties.MediaType.Enum, want) {
			t.Fatalf("artifact schema media types missing %q: %#v", want, artifactSchema.Properties.MediaType.Enum)
		}
	}
	for _, want := range []string{"source_module", "operation", "fixture"} {
		if !contains(artifactSchema.Properties.Provenance.Required, want) {
			t.Fatalf("artifact provenance missing required field %q: %#v", want, artifactSchema.Properties.Provenance.Required)
		}
	}
}

func TestFinRobotCodingNotebookDeterministicReplayAndCapabilityMetadata(t *testing.T) {
	base := codingNotebookLivePackageDir(t)
	manifest := loadCodingNotebookLiveManifest(t, base)

	metadataByCapability := map[string]codingNotebookCapabilityMetadata{}
	for _, metadata := range manifest.CapabilityMetadata {
		if metadata.ID == "" || metadata.Capability == "" || metadata.Default == "" || metadata.Schema == "" {
			t.Fatalf("capability metadata incomplete: %#v", metadata)
		}
		metadataByCapability[metadata.Capability] = metadata
	}
	for _, want := range []string{"coding.notebook.file.read", "coding.notebook.file.write", "coding.notebook.command.execute", "coding.notebook.image.display", "coding.notebook.replay.deterministic"} {
		if metadataByCapability[want].Capability == "" {
			t.Fatalf("missing capability metadata %q", want)
		}
	}
	if !metadataByCapability["coding.notebook.file.write"].ApprovalRequired ||
		!metadataByCapability["coding.notebook.command.execute"].ApprovalRequired ||
		metadataByCapability["coding.notebook.image.display"].ApprovalRequired {
		t.Fatalf("approval metadata = %#v", metadataByCapability)
	}

	var replay struct {
		ProviderFree bool `json:"provider_free"`
		LiveNetwork  bool `json:"live_network"`
		Commands     []struct {
			Approval struct {
				Decision string `json:"decision"`
			} `json:"approval"`
			Result struct {
				ExitCode int    `json:"exit_code"`
				Stdout   string `json:"stdout"`
				Stderr   string `json:"stderr"`
			} `json:"result"`
			ArtifactRefs  []string `json:"artifact_refs"`
			Deterministic bool     `json:"deterministic"`
		} `json:"commands"`
		Artifacts []struct {
			ArtifactID string `json:"artifact_id"`
			ReplayKey  string `json:"replay_key"`
			SHA256     string `json:"sha256"`
			Provenance struct {
				SourceModule string `json:"source_module"`
				Fixture      bool   `json:"fixture"`
			} `json:"provenance"`
		} `json:"artifacts"`
		Displays []struct {
			Capability   string `json:"capability"`
			MediaType    string `json:"media_type"`
			SHA256       string `json:"sha256"`
			LiveNotebook bool   `json:"live_notebook"`
		} `json:"displays"`
		ReplayAssertions struct {
			Stdout        string `json:"stdout"`
			Stderr        string `json:"stderr"`
			ExitCode      int    `json:"exit_code"`
			ArtifactCount int    `json:"artifact_count"`
			DisplayCount  int    `json:"display_count"`
			LiveNotebook  bool   `json:"live_notebook"`
		} `json:"replay_assertions"`
	}
	decodeCodingNotebookJSONFile(t, filepath.Join(base, "fixtures", "deterministic_replay_fixture.json"), &replay)
	if !replay.ProviderFree || replay.LiveNetwork || len(replay.Commands) != 1 || len(replay.Artifacts) != 3 || len(replay.Displays) != 1 {
		t.Fatalf("replay fixture header/count = %#v", replay)
	}
	command := replay.Commands[0]
	if command.Approval.Decision != "allow_fixture" || command.Result.ExitCode != 0 ||
		command.Result.Stdout != replay.ReplayAssertions.Stdout || command.Result.Stderr != replay.ReplayAssertions.Stderr ||
		len(command.ArtifactRefs) != 2 || !command.Deterministic {
		t.Fatalf("command replay is not deterministic/captured: %#v", command)
	}
	for _, artifact := range replay.Artifacts {
		if artifact.ArtifactID == "" || artifact.ReplayKey == "" || len(artifact.SHA256) != 64 ||
			artifact.Provenance.SourceModule != "finrobot/functional/coding.py" || !artifact.Provenance.Fixture {
			t.Fatalf("artifact fixture contract incomplete: %#v", artifact)
		}
	}
	display := replay.Displays[0]
	if display.Capability != "coding.notebook.image.display" || display.MediaType != "image/png" || len(display.SHA256) != 64 || display.LiveNotebook {
		t.Fatalf("display image fixture = %#v", display)
	}
	if replay.ReplayAssertions.ArtifactCount != len(replay.Artifacts) ||
		replay.ReplayAssertions.DisplayCount != len(replay.Displays) ||
		replay.ReplayAssertions.ExitCode != command.Result.ExitCode ||
		replay.ReplayAssertions.LiveNotebook {
		t.Fatalf("replay assertions = %#v", replay.ReplayAssertions)
	}
}

func codingNotebookLivePackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "coding_notebook")
}

func loadCodingNotebookLiveManifest(t *testing.T, base string) codingNotebookLiveManifest {
	t.Helper()
	var manifest codingNotebookLiveManifest
	decodeCodingNotebookJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	return manifest
}

func assertCodingNotebookJSONFile(t *testing.T, path string) {
	t.Helper()
	var value any
	decodeCodingNotebookJSONFile(t, path, &value)
}

func decodeCodingNotebookJSONFile(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}
