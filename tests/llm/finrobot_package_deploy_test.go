package leia_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type finrobotPackageDeployManifest struct {
	SchemaVersion int    `json:"schema_version"`
	GapID         string `json:"gap_id"`
	Package       struct {
		Name           string `json:"name"`
		Version        string `json:"version"`
		PythonRequires string `json:"python_requires"`
	} `json:"package"`
	Files   map[string]string `json:"files"`
	Install struct {
		BaseRequirements []string            `json:"base_requirements"`
		OptionalExtras   map[string][]string `json:"optional_extras"`
	} `json:"install"`
	Environment struct {
		Required []struct {
			Name    string `json:"name"`
			Default string `json:"default"`
		} `json:"required"`
		Optional []struct {
			Name   string `json:"name"`
			Redact bool   `json:"redact"`
		} `json:"optional"`
		Checks []struct {
			ID       string `json:"id"`
			Kind     string `json:"kind"`
			Env      string `json:"env"`
			Path     string `json:"path"`
			BaseEnv  string `json:"base_env"`
			Required bool   `json:"required"`
			Redact   bool   `json:"redact"`
		} `json:"checks"`
	} `json:"environment"`
	Commands map[string]string `json:"commands"`
	Smoke    struct {
		ProviderFree bool     `json:"provider_free"`
		Checks       []string `json:"checks"`
	} `json:"smoke"`
}

func TestFinRobotPackageDeployManifestSmoke(t *testing.T) {
	root := repoRoot(t)
	base := filepath.Join(root, "examples", "ai", "finrobot_translation")
	manifest := loadFinRobotPackageDeployManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.GapID != "FR-GAP-021" {
		t.Fatalf("manifest header = schema %d gap %q", manifest.SchemaVersion, manifest.GapID)
	}
	if manifest.Package.Name != "leia-finrobot-translation" || manifest.Package.PythonRequires != ">=3.10" {
		t.Fatalf("package metadata = %#v", manifest.Package)
	}
	if !manifest.Smoke.ProviderFree {
		t.Fatal("package/deploy smoke must be provider-free")
	}

	for key, name := range manifest.Files {
		if !strings.HasPrefix(name, "package_deploy") {
			t.Fatalf("file %s = %q, want package_deploy* name", key, name)
		}
		if _, err := os.Stat(filepath.Join(base, name)); err != nil {
			t.Fatalf("manifest file %s (%s): %v", key, name, err)
		}
	}
	for _, key := range []string{"requirements", "setup", "dockerfile", "gcloud", "run_web_app"} {
		if manifest.Files[key] == "" {
			t.Fatalf("missing manifest files.%s", key)
		}
	}
}

func TestFinRobotPackageDeployOptionalExtrasAndCommands(t *testing.T) {
	root := repoRoot(t)
	base := filepath.Join(root, "examples", "ai", "finrobot_translation")
	manifest := loadFinRobotPackageDeployManifest(t, base)

	setupData := readText(t, filepath.Join(base, manifest.Files["setup"]))
	requirementsData := readText(t, filepath.Join(base, manifest.Files["requirements"]))
	for _, req := range manifest.Install.BaseRequirements {
		if !strings.Contains(requirementsData, req) || !strings.Contains(setupData, `"`+req+`"`) {
			t.Fatalf("base requirement %q not mirrored in requirements and setup", req)
		}
	}
	for extra, deps := range manifest.Install.OptionalExtras {
		if !strings.Contains(requirementsData, "["+extra+"]") {
			t.Fatalf("requirements does not document optional extra %q", extra)
		}
		if !strings.Contains(setupData, `"`+extra+`"`) && extra != "all" {
			t.Fatalf("setup does not define optional extra %q", extra)
		}
		for _, dep := range deps {
			if strings.HasPrefix(dep, manifest.Package.Name+"[") {
				continue
			}
			if !strings.Contains(setupData, `"`+dep+`"`) {
				t.Fatalf("setup extra %q missing dependency %q", extra, dep)
			}
		}
	}

	dockerfile := readText(t, filepath.Join(base, manifest.Files["dockerfile"]))
	gcloud := readText(t, filepath.Join(base, manifest.Files["gcloud"]))
	runWebApp := readText(t, filepath.Join(base, manifest.Files["run_web_app"]))
	for _, want := range []string{manifest.Files["requirements"], manifest.Files["run_web_app"], "LEIA_FINROBOT_DATA_DIR", "PORT=8080"} {
		if !strings.Contains(dockerfile, want) {
			t.Fatalf("dockerfile missing %q", want)
		}
	}
	for _, want := range []string{manifest.Files["dockerfile"], "gcloud run deploy", "LEIA_FINROBOT_DATA_DIR"} {
		if !strings.Contains(gcloud, want) {
			t.Fatalf("gcloud script missing %q", want)
		}
	}
	for _, want := range []string{"def check_environment()", "def main()", "evaluation_harness/manifest.json"} {
		if !strings.Contains(runWebApp, want) {
			t.Fatalf("run_web_app missing %q", want)
		}
	}
	for _, name := range []string{"local_service", "docker_build", "gcloud_deploy"} {
		command := manifest.Commands[name]
		if !strings.Contains(command, "package_deploy") {
			t.Fatalf("command %s = %q, want package_deploy reference", name, command)
		}
	}
}

func TestFinRobotPackageDeployEnvironmentChecks(t *testing.T) {
	root := repoRoot(t)
	base := filepath.Join(root, "examples", "ai", "finrobot_translation")
	manifest := loadFinRobotPackageDeployManifest(t, base)

	var requiredEnv []string
	for _, env := range manifest.Environment.Required {
		requiredEnv = append(requiredEnv, env.Name)
		if env.Name == "LEIA_FINROBOT_DATA_DIR" && env.Default == "" {
			t.Fatal("LEIA_FINROBOT_DATA_DIR must document a local default")
		}
	}
	if !contains(requiredEnv, "LEIA_FINROBOT_DATA_DIR") {
		t.Fatalf("required env = %#v, want LEIA_FINROBOT_DATA_DIR", requiredEnv)
	}

	optionalSecrets := map[string]bool{}
	for _, env := range manifest.Environment.Optional {
		if strings.HasSuffix(env.Name, "_API_KEY") {
			optionalSecrets[env.Name] = env.Redact
		}
	}
	for _, key := range []string{"OPENAI_API_KEY", "FINNHUB_API_KEY", "FMP_API_KEY"} {
		if !optionalSecrets[key] {
			t.Fatalf("optional secret %s must be documented and redacted", key)
		}
	}

	checks := map[string]bool{}
	for _, check := range manifest.Environment.Checks {
		checks[check.ID] = true
		if check.Required && check.Kind == "secret" {
			t.Fatalf("secret check %s must be optional", check.ID)
		}
		if check.Redact && check.Env == "" {
			t.Fatalf("redacted check %s must name an env var", check.ID)
		}
	}
	for _, id := range []string{"data_dir_exists", "replay_manifest_present", "live_provider_keys_optional"} {
		if !checks[id] {
			t.Fatalf("missing environment check %q", id)
		}
	}
}

func loadFinRobotPackageDeployManifest(t *testing.T, base string) finrobotPackageDeployManifest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(base, "package_deploy_manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest finrobotPackageDeployManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode package_deploy_manifest.json: %v", err)
	}
	return manifest
}

func readText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
