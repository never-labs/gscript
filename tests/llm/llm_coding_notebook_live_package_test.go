package leia_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
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
	ArtifactManifests  map[string]string                  `json:"artifact_manifests"`
	CleanupPolicy      codingNotebookCleanupPolicy        `json:"cleanup_policy"`
	Capabilities       []string                           `json:"capabilities"`
	CapabilityMetadata []codingNotebookCapabilityMetadata `json:"capability_metadata"`
	TestGates          []string                           `json:"test_gates"`
	NoBuiltInGuarantee struct {
		Required bool `json:"required"`
	} `json:"no_built_in_guarantee"`
}

type codingNotebookCleanupPolicy struct {
	Mode                     string   `json:"mode"`
	DeleteUnapprovedOutputs  bool     `json:"delete_unapproved_outputs"`
	PreserveFixtureArtifacts bool     `json:"preserve_fixture_artifacts"`
	CleanupReplayKey         string   `json:"cleanup_replay_key"`
	Paths                    []string `json:"paths"`
	Reason                   string   `json:"reason"`
	Executed                 bool     `json:"executed"`
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
	if manifest.Entrypoints["smoke"] != "main.leia" {
		t.Fatalf("smoke entrypoint = %q, want main.leia", manifest.Entrypoints["smoke"])
	}
	if _, err := os.Stat(filepath.Join(base, manifest.Entrypoints["smoke"])); err != nil {
		t.Fatalf("smoke entrypoint missing: %v", err)
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
	if manifest.ArtifactManifests["file"] != "fixtures/deterministic_replay_fixture.json#/file_artifact_manifest" ||
		manifest.ArtifactManifests["image"] != "fixtures/deterministic_replay_fixture.json#/image_artifact_manifest" {
		t.Fatalf("artifact manifests = %#v", manifest.ArtifactManifests)
	}
	if manifest.CleanupPolicy.Mode != "fixture_noop" ||
		!manifest.CleanupPolicy.DeleteUnapprovedOutputs ||
		!manifest.CleanupPolicy.PreserveFixtureArtifacts ||
		manifest.CleanupPolicy.CleanupReplayKey != "cleanup:coding-notebook:fixture-noop" ||
		len(manifest.CleanupPolicy.Paths) != 2 ||
		!strings.Contains(manifest.CleanupPolicy.Reason, "without touching the host filesystem") {
		t.Fatalf("cleanup policy = %#v", manifest.CleanupPolicy)
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
	for _, want := range []string{"sandbox", "file", "image", "denied command", "command envelopes", "stdout", "stderr", "file artifact manifest", "image artifact manifest", "deterministic replay", "cleanup policy", "capability metadata"} {
		if !strings.Contains(joinedGates, want) {
			t.Fatalf("test gates missing %q: %s", want, joinedGates)
		}
	}
}

func TestFinRobotCodingNotebookLivePackageSmokeExecutes(t *testing.T) {
	base := codingNotebookLivePackageDir(t)
	manifest := loadCodingNotebookLiveManifest(t, base)
	mainPath := filepath.Join(base, manifest.Entrypoints["smoke"])
	if manifest.Entrypoints["smoke"] != "main.leia" {
		t.Fatalf("smoke entrypoint = %q, want main.leia", manifest.Entrypoints["smoke"])
	}

	src, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatal(err)
	}
	source := string(src)
	for _, want := range []string{
		"coding_notebook_live_package",
		"sandbox_approvals",
		"deny_live_notebook_kernel",
		"denied_command",
		"stdout",
		"stderr",
		"deterministic_replay",
		"command_envelope",
		"file_image_artifacts",
		"file_artifact_manifest",
		"image_artifact_manifest",
		"cleanup_policy",
		"capability_metadata",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("main.leia missing %q", want)
		}
	}
	for _, forbidden := range []string{"import q", "q/runtime", "ipykernel_launcher -f /tmp/live-kernel.json`", "$`", "$!`"} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("main.leia must stay provider-free and avoid host/q runtime execution; found %q", forbidden)
		}
	}

	vm := leia.New(leia.WithLibs(leia.LibAll), leia.WithVM())
	if err := vm.ExecFile(mainPath); err != nil {
		t.Fatalf("ExecFile main.leia: %v", err)
	}
	value, err := vm.Get("coding_notebook_live_package_summary")
	if err != nil {
		t.Fatalf("get coding_notebook_live_package_summary: %v", err)
	}
	summary := fmt.Sprint(value)
	for _, want := range []string{
		"coding_notebook_live_package",
		"approvals=3",
		"denied=deny",
		"stdout=signals: PDD=0.18 BABA=0.11",
		"stderr=denied: notebook kernel execution is disabled in replay mode",
		"replay=finrobot-coding-notebook-v1",
		"executed=false",
		"file_manifest=2",
		"image_manifest=1",
		"cleanup=fixture_noop",
		"artifacts=3",
		"displays=1",
		"capabilities=5",
		"provider_free=true",
		"live_network=false",
		"imports=false",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary missing %q: %s", want, summary)
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
			StdoutRequired                     bool `json:"stdout_required"`
			StderrRequired                     bool `json:"stderr_required"`
			ExitCodeRequired                   bool `json:"exit_code_required"`
			ApprovalDecisionRequired           bool `json:"approval_decision_required"`
			DeniedCommandsMustHaveReason       bool `json:"denied_commands_must_have_reason"`
			CommandEnvelopeRequired            bool `json:"command_envelope_required"`
			ExecutedFlagRequired               bool `json:"executed_flag_required"`
			StdoutStderrChannelRecordsRequired bool `json:"stdout_stderr_channel_records_required"`
		} `json:"capture_contract"`
		CleanupPolicy codingNotebookCleanupPolicy `json:"cleanup_policy"`
	}
	decodeCodingNotebookJSONFile(t, filepath.Join(base, "contracts", "sandbox_policy_gates.json"), &policy)
	if !policy.ProviderFree || policy.LiveNetwork || policy.RealDependencyImports || policy.DefaultDecision != "deny" {
		t.Fatalf("policy header/default decision = %#v", policy)
	}
	if !policy.CaptureContract.StdoutRequired || !policy.CaptureContract.StderrRequired || !policy.CaptureContract.ExitCodeRequired ||
		!policy.CaptureContract.ApprovalDecisionRequired || !policy.CaptureContract.DeniedCommandsMustHaveReason ||
		!policy.CaptureContract.CommandEnvelopeRequired || !policy.CaptureContract.ExecutedFlagRequired ||
		!policy.CaptureContract.StdoutStderrChannelRecordsRequired {
		t.Fatalf("capture contract incomplete: %#v", policy.CaptureContract)
	}
	if policy.CleanupPolicy.Mode != "fixture_noop" ||
		!policy.CleanupPolicy.DeleteUnapprovedOutputs ||
		!policy.CleanupPolicy.PreserveFixtureArtifacts ||
		policy.CleanupPolicy.CleanupReplayKey != "cleanup:coding-notebook:fixture-noop" {
		t.Fatalf("policy cleanup = %#v", policy.CleanupPolicy)
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
		CommandEnvelope struct {
			Argv            []string `json:"argv"`
			CWD             string   `json:"cwd"`
			EnvAllowlist    []string `json:"env_allowlist"`
			Shell           bool     `json:"shell"`
			TimeoutBucket   string   `json:"timeout_bucket"`
			Network         bool     `json:"network"`
			FilesystemWrite bool     `json:"filesystem_write"`
			Executed        bool     `json:"executed"`
		} `json:"command_envelope"`
		Result struct {
			ExitCode       int    `json:"exit_code"`
			Stdout         string `json:"stdout"`
			Stderr         string `json:"stderr"`
			DurationBucket string `json:"duration_bucket"`
		} `json:"result"`
		Captures []struct {
			Channel   string `json:"channel"`
			Text      string `json:"text"`
			Bytes     int    `json:"bytes"`
			Truncated bool   `json:"truncated"`
		} `json:"captures"`
		Executed      bool `json:"executed"`
		Deterministic bool `json:"deterministic"`
	}
	decodeCodingNotebookJSONFile(t, filepath.Join(base, "fixtures", "denied_command_fixture.json"), &denied)
	if !denied.ProviderFree || denied.LiveNetwork || denied.ReplayKey != "command:deny:notebook-kernel" ||
		denied.Capability != "coding.notebook.command.execute" || denied.Approval.Decision != "deny" ||
		denied.Approval.Reason == "" || denied.Result.ExitCode == 0 || denied.Result.Stdout != "" ||
		!strings.Contains(denied.Result.Stderr, "denied") || denied.Result.DurationBucket != "denied" ||
		len(denied.CommandEnvelope.Argv) == 0 || denied.CommandEnvelope.Shell || denied.CommandEnvelope.Network ||
		denied.CommandEnvelope.FilesystemWrite || denied.CommandEnvelope.Executed || denied.Executed ||
		!denied.Deterministic {
		t.Fatalf("denied command fixture = %#v", denied)
	}
	assertCodingNotebookCaptureChannels(t, denied.Captures, denied.Result.Stdout, denied.Result.Stderr)

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
			CommandEnvelope struct {
				Argv            []string `json:"argv"`
				Shell           bool     `json:"shell"`
				TimeoutBucket   string   `json:"timeout_bucket"`
				Network         bool     `json:"network"`
				FilesystemWrite bool     `json:"filesystem_write"`
				Executed        bool     `json:"executed"`
			} `json:"command_envelope"`
			Result struct {
				ExitCode       int    `json:"exit_code"`
				Stdout         string `json:"stdout"`
				Stderr         string `json:"stderr"`
				DurationBucket string `json:"duration_bucket"`
			} `json:"result"`
			Captures []struct {
				Channel   string `json:"channel"`
				Text      string `json:"text"`
				Bytes     int    `json:"bytes"`
				Truncated bool   `json:"truncated"`
			} `json:"captures"`
			ArtifactRefs  []string `json:"artifact_refs"`
			Executed      bool     `json:"executed"`
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
		FileArtifactManifest struct {
			ManifestID  string   `json:"manifest_id"`
			ReplayKey   string   `json:"replay_key"`
			Kind        string   `json:"kind"`
			ArtifactIDs []string `json:"artifact_ids"`
			Count       int      `json:"count"`
			TotalBytes  int      `json:"total_bytes"`
			SHA256      string   `json:"sha256"`
		} `json:"file_artifact_manifest"`
		ImageArtifactManifest struct {
			ManifestID  string   `json:"manifest_id"`
			ReplayKey   string   `json:"replay_key"`
			Kind        string   `json:"kind"`
			ArtifactIDs []string `json:"artifact_ids"`
			Count       int      `json:"count"`
			TotalBytes  int      `json:"total_bytes"`
			SHA256      string   `json:"sha256"`
			Displays    []string `json:"displays"`
		} `json:"image_artifact_manifest"`
		ReplayAssertions struct {
			Stdout                     string `json:"stdout"`
			Stderr                     string `json:"stderr"`
			ExitCode                   int    `json:"exit_code"`
			DurationBucket             string `json:"duration_bucket"`
			ArtifactCount              int    `json:"artifact_count"`
			FileArtifactManifestCount  int    `json:"file_artifact_manifest_count"`
			ImageArtifactManifestCount int    `json:"image_artifact_manifest_count"`
			DisplayCount               int    `json:"display_count"`
			LiveNotebook               bool   `json:"live_notebook"`
			Executed                   bool   `json:"executed"`
		} `json:"replay_assertions"`
		CleanupPolicy codingNotebookCleanupPolicy `json:"cleanup_policy"`
	}
	decodeCodingNotebookJSONFile(t, filepath.Join(base, "fixtures", "deterministic_replay_fixture.json"), &replay)
	if !replay.ProviderFree || replay.LiveNetwork || len(replay.Commands) != 1 || len(replay.Artifacts) != 3 || len(replay.Displays) != 1 {
		t.Fatalf("replay fixture header/count = %#v", replay)
	}
	command := replay.Commands[0]
	if command.Approval.Decision != "allow_fixture" || command.Result.ExitCode != 0 ||
		command.Result.Stdout != replay.ReplayAssertions.Stdout || command.Result.Stderr != replay.ReplayAssertions.Stderr ||
		command.Result.DurationBucket != replay.ReplayAssertions.DurationBucket ||
		len(command.ArtifactRefs) != 2 || len(command.CommandEnvelope.Argv) == 0 || command.CommandEnvelope.Shell ||
		command.CommandEnvelope.Network || command.CommandEnvelope.FilesystemWrite || command.CommandEnvelope.Executed ||
		command.Executed || !command.Deterministic {
		t.Fatalf("command replay is not deterministic/captured: %#v", command)
	}
	assertCodingNotebookCaptureChannels(t, command.Captures, command.Result.Stdout, command.Result.Stderr)
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
	if replay.FileArtifactManifest.ManifestID != "manifest:file:strategy-fixtures" ||
		replay.FileArtifactManifest.Kind != "file" ||
		replay.FileArtifactManifest.Count != 2 ||
		len(replay.FileArtifactManifest.ArtifactIDs) != replay.FileArtifactManifest.Count ||
		len(replay.FileArtifactManifest.SHA256) != 64 {
		t.Fatalf("file artifact manifest = %#v", replay.FileArtifactManifest)
	}
	if replay.ImageArtifactManifest.ManifestID != "manifest:image:allocation-fixtures" ||
		replay.ImageArtifactManifest.Kind != "image" ||
		replay.ImageArtifactManifest.Count != 1 ||
		len(replay.ImageArtifactManifest.ArtifactIDs) != replay.ImageArtifactManifest.Count ||
		len(replay.ImageArtifactManifest.Displays) != 1 ||
		len(replay.ImageArtifactManifest.SHA256) != 64 {
		t.Fatalf("image artifact manifest = %#v", replay.ImageArtifactManifest)
	}
	if replay.ReplayAssertions.ArtifactCount != len(replay.Artifacts) ||
		replay.ReplayAssertions.FileArtifactManifestCount != replay.FileArtifactManifest.Count ||
		replay.ReplayAssertions.ImageArtifactManifestCount != replay.ImageArtifactManifest.Count ||
		replay.ReplayAssertions.DisplayCount != len(replay.Displays) ||
		replay.ReplayAssertions.ExitCode != command.Result.ExitCode ||
		replay.ReplayAssertions.LiveNotebook ||
		replay.ReplayAssertions.Executed {
		t.Fatalf("replay assertions = %#v", replay.ReplayAssertions)
	}
	if replay.CleanupPolicy.Mode != "fixture_noop" ||
		!replay.CleanupPolicy.DeleteUnapprovedOutputs ||
		!replay.CleanupPolicy.PreserveFixtureArtifacts ||
		replay.CleanupPolicy.CleanupReplayKey != "cleanup:coding-notebook:fixture-noop" ||
		len(replay.CleanupPolicy.Paths) != 2 ||
		replay.CleanupPolicy.Executed {
		t.Fatalf("replay cleanup policy = %#v", replay.CleanupPolicy)
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

func assertCodingNotebookCaptureChannels(t *testing.T, captures []struct {
	Channel   string `json:"channel"`
	Text      string `json:"text"`
	Bytes     int    `json:"bytes"`
	Truncated bool   `json:"truncated"`
}, stdout, stderr string) {
	t.Helper()
	if len(captures) != 2 {
		t.Fatalf("capture channel count = %d, want 2", len(captures))
	}
	byChannel := map[string]struct {
		Text      string
		Bytes     int
		Truncated bool
	}{}
	for _, capture := range captures {
		if capture.Channel != "stdout" && capture.Channel != "stderr" {
			t.Fatalf("unexpected capture channel: %#v", capture)
		}
		byChannel[capture.Channel] = struct {
			Text      string
			Bytes     int
			Truncated bool
		}{Text: capture.Text, Bytes: capture.Bytes, Truncated: capture.Truncated}
	}
	if byChannel["stdout"].Text != stdout || byChannel["stderr"].Text != stderr {
		t.Fatalf("capture text mismatch: captures=%#v stdout=%q stderr=%q", captures, stdout, stderr)
	}
	if byChannel["stdout"].Bytes != len(stdout) || byChannel["stderr"].Bytes != len(stderr) {
		t.Fatalf("capture byte count mismatch: captures=%#v", captures)
	}
	if byChannel["stdout"].Truncated || byChannel["stderr"].Truncated {
		t.Fatalf("captures must not be truncated: %#v", captures)
	}
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
