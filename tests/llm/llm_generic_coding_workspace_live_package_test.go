package leia_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestGenericCodingWorkspaceLivePackageContractFixtureClosedLoop(t *testing.T) {
	base := genericCodingWorkspacePackageDir(t)
	var manifest struct {
		SchemaVersion      int               `json:"schema_version"`
		ID                 string            `json:"id"`
		PackageName        string            `json:"package_name"`
		PackageBoundaryID  string            `json:"package_boundary_id"`
		CapabilityID       string            `json:"capability_id"`
		ProviderFree       bool              `json:"provider_free"`
		DomainSpecific     bool              `json:"domain_specific"`
		LiveNetworkDefault bool              `json:"live_network_default"`
		LiveModelDefault   bool              `json:"live_model_default"`
		DependsOnQRuntime  bool              `json:"depends_on_q_runtime"`
		Capabilities       []string          `json:"capabilities"`
		Contracts          map[string]string `json:"contracts"`
		Schemas            map[string]string `json:"schemas"`
		Fixtures           map[string]string `json:"fixtures"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	if manifest.SchemaVersion != 1 || manifest.ID != "generic-coding-workspace" ||
		manifest.PackageName != "leia-generic-ai-coding-workspace" ||
		manifest.PackageBoundaryID != "generic-ai-coding-workspace" ||
		manifest.CapabilityID != "generic.ai.coding_workspace" {
		t.Fatalf("unexpected manifest identity: %#v", manifest)
	}
	if !manifest.ProviderFree || manifest.DomainSpecific || manifest.LiveNetworkDefault || manifest.LiveModelDefault || manifest.DependsOnQRuntime {
		t.Fatalf("manifest must stay provider-free/generic/offline: %#v", manifest)
	}
	for _, want := range []string{"generic.ai.coding_workspace", "generic.ai.code.command.request", "generic.ai.code.command.result", "generic.ai.code.approval.gate", "generic.ai.code.stdout.stderr", "generic.ai.artifact.file_manifest", "generic.ai.artifact.image_manifest", "generic.ai.notebook.display", "generic.ai.code.cleanup_policy", "generic.ai.code.clean_skip"} {
		if !genericLivePackageContains(manifest.Capabilities, want) {
			t.Fatalf("manifest capabilities missing %q: %#v", want, manifest.Capabilities)
		}
	}

	var contract struct {
		SchemaVersion         int               `json:"schema_version"`
		PackageBoundaryID     string            `json:"package_boundary_id"`
		PackageName           string            `json:"package_name"`
		Entrypoint            string            `json:"entrypoint"`
		ProviderFree          bool              `json:"provider_free"`
		DomainSpecific        bool              `json:"domain_specific"`
		LiveNetwork           bool              `json:"live_network"`
		LiveModelCalls        bool              `json:"live_model_calls"`
		RealDependencyImports bool              `json:"real_dependency_imports"`
		FieldContracts        map[string]string `json:"field_contracts"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, manifest.Contracts["contract"]), &contract)
	if contract.SchemaVersion != 1 || contract.PackageBoundaryID != manifest.PackageBoundaryID ||
		contract.PackageName != "generic.ai.coding_workspace" || contract.Entrypoint != "ai.coding_workspace" ||
		!contract.ProviderFree || contract.DomainSpecific || contract.LiveNetwork ||
		contract.LiveModelCalls || contract.RealDependencyImports {
		t.Fatalf("contract boundary mismatch: %#v", contract)
	}
	for _, want := range []string{"sandbox_command", "approval_gate", "stdout_stderr_capture", "file_artifact_manifest", "image_artifact_manifest", "notebook_display", "cleanup_policy"} {
		if contract.FieldContracts[want] == "" {
			t.Fatalf("contract field_contracts missing %q: %#v", want, contract.FieldContracts)
		}
	}
}

func TestGenericCodingWorkspaceLivePackageFixtureShape(t *testing.T) {
	base := genericCodingWorkspacePackageDir(t)
	fixture := loadGenericCodingWorkspaceFixture(t, filepath.Join(base, "fixtures", "generic_coding_workspace_fixture.json"))
	if !fixture.ProviderFree || fixture.LiveNetwork || fixture.RealDependencyImports || fixture.LiveModelCalls {
		t.Fatalf("fixture must stay provider-free and offline: %#v", fixture)
	}
	if len(fixture.ApprovalGates) != 2 || len(fixture.Commands) != 2 || len(fixture.ChannelCaptures) != 3 ||
		len(fixture.Artifacts) != 3 || len(fixture.NotebookDisplays) != 1 || len(fixture.AdapterBoundaries) != 2 {
		t.Fatalf("fixture counts drifted: approvals=%d commands=%d captures=%d artifacts=%d displays=%d adapters=%d",
			len(fixture.ApprovalGates), len(fixture.Commands), len(fixture.ChannelCaptures), len(fixture.Artifacts), len(fixture.NotebookDisplays), len(fixture.AdapterBoundaries))
	}
	approvals := map[string]bool{}
	for _, gate := range fixture.ApprovalGates {
		if gate.ID == "" || gate.Decision == "" || gate.Reason == "" {
			t.Fatalf("approval gate incomplete: %#v", gate)
		}
		approvals[gate.ID] = true
	}
	commandKeys := map[string]bool{}
	for _, command := range fixture.Commands {
		if command.RequestID == "" || command.ReplayKey == "" || len(command.Argv) == 0 || command.CWD == "" || !approvals[command.ApprovalID] {
			t.Fatalf("command incomplete or approval does not resolve: %#v", command)
		}
		if command.Network || command.FilesystemWrite || command.Executed {
			t.Fatalf("fixture command must not perform side effects: %#v", command)
		}
		commandKeys[command.ReplayKey] = true
	}
	for _, capture := range fixture.ChannelCaptures {
		if !commandKeys[capture.CommandReplayKey] || capture.Channel == "" || capture.Bytes < 0 {
			t.Fatalf("channel capture incomplete or command does not resolve: %#v", capture)
		}
	}
	artifacts := map[string]bool{}
	for _, artifact := range fixture.Artifacts {
		if artifact.ArtifactID == "" || artifact.ReplayKey == "" || artifact.Path == "" || artifact.MediaType == "" || artifact.SHA256 == "" || artifact.Bytes <= 0 {
			t.Fatalf("artifact incomplete: %#v", artifact)
		}
		artifacts[artifact.ArtifactID] = true
	}
	for _, display := range fixture.NotebookDisplays {
		if !artifacts[display.ArtifactID] || display.LiveKernel || display.Width <= 0 || display.Height <= 0 {
			t.Fatalf("display incomplete or imports live kernel: %#v", display)
		}
	}
	if fixture.CleanupPolicy.Mode != "fixture_noop" || fixture.CleanupPolicy.Executed {
		t.Fatalf("cleanup policy must be fixture-only: %#v", fixture.CleanupPolicy)
	}
	for _, boundary := range fixture.AdapterBoundaries {
		if boundary.DependencyImported || boundary.CredentialRequired || boundary.LiveNetwork || !boundary.CleanSkip {
			t.Fatalf("adapter boundary must clean-skip: %#v", boundary)
		}
	}
}

func TestGenericCodingWorkspaceLivePackageIsDomainNeutral(t *testing.T) {
	base := genericCodingWorkspacePackageDir(t)
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lower := strings.ToLower(string(data))
		for _, forbidden := range []string{"finrobot", "acme", "aapl", "ticker", "equity", "investment", "valuation", "dcf", "sec.gov", "10-k", "finance.", "product.workflow"} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("%s leaks domain-specific marker %q", path, forbidden)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGenericCodingWorkspaceLivePackageSchemaRequiredFields(t *testing.T) {
	base := genericCodingWorkspacePackageDir(t)
	schema := filepath.Join(base, "schemas", "generic_coding_workspace_v1.schema.json")
	assertDocumentPipelineSchemaRequired(t, schema, []string{"schema_version", "provider_free", "live_network", "real_dependency_imports", "live_model_calls", "approval_gates", "commands", "channel_captures", "file_artifact_manifest", "image_artifact_manifest", "artifacts", "notebook_displays", "cleanup_policy", "adapter_boundaries"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "commands", "items"}, []string{"request_id", "replay_key", "argv", "cwd", "env_allowlist", "shell", "timeout_bucket", "network", "filesystem_write", "executed", "approval_id", "exit_code", "stdout", "stderr", "duration_bucket"})
	assertDocumentPipelineNestedSchemaRequired(t, schema, []string{"properties", "artifacts", "items"}, []string{"artifact_id", "replay_key", "path", "media_type", "sha256", "bytes", "provenance"})
}

func TestGenericCodingWorkspaceLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(genericCodingWorkspacePackageDir(t), "main.leia")
	for _, tc := range []struct {
		name string
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var prints []string
			vm := leia.New(append([]leia.Option{
				leia.WithLibs(leia.LibString),
				leia.WithPrint(func(args ...any) {
					var parts []string
					for _, arg := range args {
						parts = append(parts, fmt.Sprint(arg))
					}
					prints = append(prints, strings.Join(parts, " "))
				}),
			}, tc.opts...)...)
			if err := vm.ExecFile(path); err != nil {
				t.Fatalf("ExecFile: %v", err)
			}
			got, err := vm.Get("generic_coding_workspace_live_package_summary")
			if err != nil {
				t.Fatalf("Get summary: %v", err)
			}
			want := "generic_coding_workspace_live_package capability=generic.ai.coding_workspace entrypoint=ai.coding_workspace approvals=2 commands=2 captures=3 artifacts=3 displays=1 clean_skip=2 provider_free=true live_network=false imports=false model_calls=false"
			if got != want {
				t.Fatalf("summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

type genericCodingWorkspaceFixture struct {
	ProviderFree          bool `json:"provider_free"`
	LiveNetwork           bool `json:"live_network"`
	RealDependencyImports bool `json:"real_dependency_imports"`
	LiveModelCalls        bool `json:"live_model_calls"`
	ApprovalGates         []struct {
		ID       string `json:"id"`
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	} `json:"approval_gates"`
	Commands []struct {
		RequestID       string   `json:"request_id"`
		ReplayKey       string   `json:"replay_key"`
		Argv            []string `json:"argv"`
		CWD             string   `json:"cwd"`
		Network         bool     `json:"network"`
		FilesystemWrite bool     `json:"filesystem_write"`
		Executed        bool     `json:"executed"`
		ApprovalID      string   `json:"approval_id"`
	} `json:"commands"`
	ChannelCaptures []struct {
		CommandReplayKey string `json:"command_replay_key"`
		Channel          string `json:"channel"`
		Bytes            int    `json:"bytes"`
	} `json:"channel_captures"`
	Artifacts []struct {
		ArtifactID string `json:"artifact_id"`
		ReplayKey  string `json:"replay_key"`
		Path       string `json:"path"`
		MediaType  string `json:"media_type"`
		SHA256     string `json:"sha256"`
		Bytes      int    `json:"bytes"`
	} `json:"artifacts"`
	NotebookDisplays []struct {
		ArtifactID string `json:"artifact_id"`
		Width      int    `json:"width"`
		Height     int    `json:"height"`
		LiveKernel bool   `json:"live_kernel"`
	} `json:"notebook_displays"`
	CleanupPolicy struct {
		Mode     string `json:"mode"`
		Executed bool   `json:"executed"`
	} `json:"cleanup_policy"`
	AdapterBoundaries []struct {
		DependencyImported bool `json:"dependency_imported"`
		CredentialRequired bool `json:"credential_required"`
		LiveNetwork        bool `json:"live_network"`
		CleanSkip          bool `json:"clean_skip"`
	} `json:"adapter_boundaries"`
}

func loadGenericCodingWorkspaceFixture(t *testing.T, path string) genericCodingWorkspaceFixture {
	t.Helper()
	var fixture genericCodingWorkspaceFixture
	decodeDocumentPipelineJSONFile(t, path, &fixture)
	return fixture
}

func genericCodingWorkspacePackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_coding_workspace")
}
