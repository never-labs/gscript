package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestExamplesCommandListsRepositoryExamples(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{"list"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExamplesCommand code = %d, stderr = %q", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"repo-hello-counter",
		"repo-embedding-go-doc-examples",
		"repo-embedding-hot_reload_project",
		"repo-site-static_docs_generator",
		"repo-tooling-package_manager_workflow-main",
		"repo-security-supply_chain_audit",
		"repo-security-vendor_onboarding_audit",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("examples list missing %q\n%s", want, out)
		}
	}
}

func TestExamplesCommandListsJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{"--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExamplesCommand code = %d, stderr = %q", code, stderr.String())
	}
	var payload struct {
		SchemaVersion int          `json:"schema_version"`
		Status        string       `json:"status"`
		ExampleCount  int          `json:"example_count"`
		Examples      []cliExample `json:"examples"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("invalid examples JSON: %v\n%s", err, stdout.String())
	}
	if payload.SchemaVersion != 1 || payload.Status != "pass" || payload.ExampleCount != len(payload.Examples) {
		t.Fatalf("examples JSON = %+v, want schema v1 pass report with matching example_count", payload)
	}
	if len(payload.Examples) == 0 {
		t.Fatal("examples JSON is empty")
	}
}

func TestExamplesCommandDiscoversDialectExamples(t *testing.T) {
	root := filepath.Dir(playgroundExamplesRoot())
	dialectDir := filepath.Join(root, "examples", "dialects")
	entries, err := os.ReadDir(dialectDir)
	if err != nil {
		t.Fatal(err)
	}

	examples, err := cliRepositoryExamples()
	if err != nil {
		t.Fatal(err)
	}
	discovered := make(map[string]bool, len(examples))
	for _, example := range examples {
		discovered[example.Path] = true
	}

	var missing []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".leia" {
			continue
		}
		path := filepath.ToSlash(filepath.Join("examples", "dialects", entry.Name()))
		if !discovered[path] {
			missing = append(missing, path)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("examples CLI must discover runnable dialect gate inputs; missing %s", strings.Join(missing, ", "))
	}
}

func TestExamplesCommandDiscoversPackageManagedProjectEntrypoints(t *testing.T) {
	root := filepath.Dir(playgroundExamplesRoot())
	examples, err := cliRepositoryExamples()
	if err != nil {
		t.Fatal(err)
	}
	discovered := make(map[string]bool, len(examples))
	for _, example := range examples {
		discovered[example.Path] = true
	}

	var projectRoots []string
	if err := filepath.WalkDir(filepath.Join(root, "examples"), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if _, err := os.Stat(filepath.Join(path, "leia.mod")); err != nil {
			return nil
		}
		if _, err := os.Stat(filepath.Join(path, "main.leia")); err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if exampleUsesOptionalQExtension(filepath.ToSlash(rel) + "/main.leia") {
			return nil
		}
		projectRoots = append(projectRoots, rel)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(projectRoots) == 0 {
		t.Fatal("no package-managed example projects found")
	}
	sort.Strings(projectRoots)
	for _, projectRoot := range projectRoots {
		t.Run(filepath.ToSlash(projectRoot), func(t *testing.T) {
			if _, err := os.Stat(filepath.Join(root, projectRoot, "leia.mod")); err != nil {
				t.Fatalf("project manifest missing: %v", err)
			}
			path := filepath.ToSlash(filepath.Join(projectRoot, "main.leia"))
			if !discovered[path] {
				t.Fatalf("examples CLI must discover package-managed project entrypoint %s", path)
			}
		})
	}
}

func TestExamplesCommandVerifiesPackageManagedProjects(t *testing.T) {
	root := filepath.Dir(playgroundExamplesRoot())
	examples, err := cliRepositoryExamples()
	if err != nil {
		t.Fatal(err)
	}
	discovered := make(map[string]cliExample, len(examples))
	for _, example := range examples {
		discovered[example.Path] = example
	}

	var moduleRoots []string
	if err := filepath.WalkDir(filepath.Join(root, "examples"), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "vendor" || strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != "leia.mod" {
			return nil
		}
		rel, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		moduleRoot := filepath.ToSlash(rel)
		if exampleUsesOptionalQExtension(moduleRoot + "/main.leia") {
			return nil
		}
		moduleRoots = append(moduleRoots, moduleRoot)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(moduleRoots) == 0 {
		t.Fatal("no package-managed example projects found")
	}
	sort.Strings(moduleRoots)

	for _, moduleRoot := range moduleRoots {
		t.Run(moduleRoot, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := runModCommand([]string{"verify", "--json", filepath.Join(root, filepath.FromSlash(moduleRoot))}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("mod verify %s code = %d stdout = %q stderr = %q", moduleRoot, code, stdout.String(), stderr.String())
			}
			var report modVerifyReport
			if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
				t.Fatalf("mod verify %s did not return JSON: %v\n%s", moduleRoot, err, stdout.String())
			}
			if !report.OK {
				t.Fatalf("mod verify %s report = %+v", moduleRoot, report)
			}
			mainPath := filepath.ToSlash(filepath.Join(moduleRoot, "main.leia"))
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(mainPath))); err == nil {
				if _, ok := discovered[mainPath]; !ok {
					t.Fatalf("package-managed project main is not discoverable by examples CLI: %s", mainPath)
				}
			}
		})
	}
}

func TestExamplesCommandVerifiesPackageManagerWorkflowProject(t *testing.T) {
	root := filepath.Dir(playgroundExamplesRoot())
	dir := filepath.Join(root, "examples", "tooling", "package_manager_workflow")

	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{"check", "--timeout=20s", "tooling/package_manager_workflow"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("examples check package_manager_workflow code = %d, stdout = %q stderr = %q", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{
		"ok      repo-tooling-package_manager_workflow-main",
		"ok      repo-tooling-package_manager_workflow-local-metadata-report",
		"examples: 2 ok, 0 skipped, 0 failed",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("examples check missing %q\n%s", want, stdout.String())
		}
	}

	stdout.Reset()
	stderr.Reset()
	code = runModCommand([]string{"graph", "--json", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mod graph package_manager_workflow code = %d, stdout = %q stderr = %q", code, stdout.String(), stderr.String())
	}
	var graph modGraphReport
	if err := json.Unmarshal(stdout.Bytes(), &graph); err != nil {
		t.Fatalf("stdout is not JSON graph: %v; stdout = %q", err, stdout.String())
	}
	if len(graph.Files) != 1 ||
		graph.Files[0].File != "main.leia" ||
		!containsString(graph.Files[0].Requires, "github.com/never-labs/leia-package-workflow/metadata/report") {
		t.Fatalf("graph = %+v, want root Go-style import edge and local replace source excluded", graph)
	}

	stdout.Reset()
	stderr.Reset()
	code = runModCommand([]string{"verify", "--json", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mod verify package_manager_workflow code = %d, stdout = %q stderr = %q", code, stdout.String(), stderr.String())
	}
	var verify modVerifyReport
	if err := json.Unmarshal(stdout.Bytes(), &verify); err != nil {
		t.Fatalf("stdout is not JSON verify report: %v; stdout = %q", err, stdout.String())
	}
	if !verify.OK {
		t.Fatalf("verify = %+v, want package_manager_workflow to verify", verify)
	}

	stdout.Reset()
	stderr.Reset()
	code = runModCommand([]string{"capability", "--json", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mod capability package_manager_workflow code = %d, stdout = %q stderr = %q", code, stdout.String(), stderr.String())
	}
	var caps modCapabilityReport
	if err := json.Unmarshal(stdout.Bytes(), &caps); err != nil {
		t.Fatalf("stdout is not JSON capability report: %v; stdout = %q", err, stdout.String())
	}
	if !caps.OK {
		t.Fatalf("capability report = %+v, want package_manager_workflow capabilities to report", caps)
	}
	if !moduleHasCapabilities(caps.Modules, "example.com/leia/examples/tooling/package-manager-workflow", "module.graph", "module.verify", "module.capability") {
		t.Fatalf("capability modules = %+v, want root module workflow capabilities", caps.Modules)
	}
	if !moduleHasCapabilities(caps.Modules, "github.com/never-labs/leia-package-workflow/metadata", "module.metadata", "module.graph", "module.verify", "module.capability") {
		t.Fatalf("capability modules = %+v, want local replace metadata module capabilities", caps.Modules)
	}
}

func TestExamplesCommandDirectorySelectorsCoverExampleProjects(t *testing.T) {
	root := filepath.Dir(playgroundExamplesRoot())
	entries, err := os.ReadDir(filepath.Join(root, "examples"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.ToSlash(filepath.Join("examples", entry.Name()))
		if !directoryHasDefaultCLIExample(filepath.Join(root, dir)) {
			continue
		}
		t.Run(dir, func(t *testing.T) {
			matches, err := selectedCLIExamples([]string{dir})
			if err != nil {
				t.Fatalf("select examples in %s: %v", dir, err)
			}
			if len(matches) == 0 {
				t.Fatalf("directory selector %s matched no examples", dir)
			}
			for _, example := range matches {
				if !strings.HasPrefix(example.Path, dir+"/") {
					t.Fatalf("directory selector %s returned %s", dir, example.Path)
				}
			}
		})
	}
}

func directoryHasDefaultCLIExample(dir string) bool {
	hasExample := false
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || hasExample {
			return nil
		}
		if d.IsDir() {
			if d.Name() == "testdata" || d.Name() == "vendor" || strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".leia" {
			return nil
		}
		rel, relErr := filepath.Rel(filepath.Dir(playgroundExamplesRoot()), path)
		if relErr != nil || exampleUsesOptionalQExtension(filepath.ToSlash(rel)) {
			return nil
		}
		hasExample = true
		return nil
	})
	return hasExample
}

func TestExamplesCommandJSONCaptureBlocksProcessOutputLeaks(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := captureCLIExampleProcessOutput(func() int {
		_, _ = os.Stdout.WriteString("direct stdout leak\n")
		_, _ = os.Stderr.WriteString("direct stderr leak\n")
		return 0
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("capture code = %d, want 0", code)
	}
	if stdout.String() != "direct stdout leak\n" {
		t.Fatalf("captured stdout = %q", stdout.String())
	}
	if stderr.String() != "direct stderr leak\n" {
		t.Fatalf("captured stderr = %q", stderr.String())
	}
}

func TestExamplesDocsIndexCoversTopLevelExampleDirectories(t *testing.T) {
	root := filepath.Dir(playgroundExamplesRoot())
	data, err := os.ReadFile(filepath.Join(root, "docs", "examples", "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	index := string(data)

	entries, err := os.ReadDir(filepath.Join(root, "examples"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		children, err := os.ReadDir(filepath.Join(root, "examples", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if len(children) == 0 {
			continue
		}
		dir := "examples/" + entry.Name() + "/"
		if !strings.Contains(index, "`"+dir+"`") {
			t.Fatalf("docs/examples/index.md is missing top-level example directory %s", dir)
		}
	}
}

func TestExamplesDocsIndexCommandsReferenceRegisteredExamples(t *testing.T) {
	root := filepath.Dir(playgroundExamplesRoot())
	data, err := os.ReadFile(filepath.Join(root, "docs", "examples", "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	commands := documentedExamplesCommands(string(data))
	if len(commands) == 0 {
		t.Fatal("docs/examples/index.md must document at least one leia examples command")
	}
	for _, command := range commands {
		t.Run(strings.Join(command, " "), func(t *testing.T) {
			switch command[0] {
			case "list":
				if len(command) > 1 && command[1] != "--json" {
					t.Fatalf("unsupported documented examples list command: %q", strings.Join(command, " "))
				}
			case "show", "run":
				if len(command) != 2 {
					t.Fatalf("documented examples %s command must have exactly one selector: %q", command[0], strings.Join(command, " "))
				}
				example, _, ok, err := resolveCLIExample(command[1])
				if err != nil {
					t.Fatal(err)
				}
				if !ok {
					t.Fatalf("documented examples %s selector is not registered: %s", command[0], command[1])
				}
				if command[0] == "run" && !example.Runnable {
					t.Fatalf("documented examples run selector is not runnable: %s", command[1])
				}
			case "check":
				selectors := documentedExamplesCheckSelectors(command)
				if len(selectors) == 0 {
					t.Fatalf("documented examples check command must select at least one example: %q", strings.Join(command, " "))
				}
				for _, selector := range selectors {
					selected, err := selectedCLIExamples([]string{selector})
					if err != nil {
						t.Fatalf("documented examples check selector is not registered: %s: %v", selector, err)
					}
					for _, example := range selected {
						if !example.Runnable && !example.Checkable {
							t.Fatalf("documented examples check selector %s matched non-checkable example %s", selector, example.ID)
						}
					}
				}
			default:
				t.Fatalf("unsupported documented examples command: %q", strings.Join(command, " "))
			}
		})
	}

	mentionedIDs := map[string]bool{}
	for _, command := range commands {
		for _, token := range command[1:] {
			if strings.HasPrefix(token, "repo-") {
				mentionedIDs[token] = true
			}
		}
	}
	for _, requiredID := range []string{
		"repo-dialects-sql_result_analytics",
		"repo-concurrency-sync_group",
		"repo-web-route_workbench",
		"repo-web-serve_dialect_app",
		"repo-web-tiny_fullstack_app",
	} {
		if !mentionedIDs[requiredID] {
			t.Fatalf("docs/examples/index.md must keep a registered command for %s", requiredID)
		}
	}
}

func TestExamplesDocsIndexKeepsDataProjectWorkflowGate(t *testing.T) {
	root := filepath.Dir(playgroundExamplesRoot())
	data, err := os.ReadFile(filepath.Join(root, "docs", "examples", "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	index := string(data)
	commands := documentedExamplesCommands(index)

	for _, selector := range []string{
		"examples/data_processing/data_oriented/soa_kernels.leia",
		"examples/data_processing/data_oriented/dense_matrix_vec_kernels.leia",
		"examples/scientific/kalman_filter.leia",
		"examples/database/package_managed",
	} {
		if !documentedExamplesCommandsContainSelector(commands, selector) {
			t.Fatalf("docs/examples/index.md must keep a registered data workflow command for %s", selector)
		}
	}

	for _, snippet := range []string{
		"SQLite `db.frame`",
		"SoA kernels",
		"dense arrays",
		"`xlsx`/`excel`",
		"round-tripping",
	} {
		if !strings.Contains(index, snippet) {
			t.Fatalf("docs/examples/index.md must keep data workflow coverage text %q", snippet)
		}
	}
}

func documentedExamplesCommandsContainSelector(commands [][]string, selector string) bool {
	for _, command := range commands {
		for _, token := range command[1:] {
			if token == selector {
				return true
			}
		}
	}
	return false
}

func documentedExamplesCommands(markdown string) [][]string {
	commandRE := regexp.MustCompile(`^go run \./cmd/leia examples (.+)$`)
	var commands [][]string
	inFence := false
	for _, line := range strings.Split(markdown, "\n") {
		if strings.HasPrefix(line, "```") {
			if inFence {
				inFence = false
				continue
			}
			info := strings.TrimSpace(strings.TrimPrefix(line, "```"))
			inFence = info == "bash"
			continue
		}
		if !inFence {
			continue
		}
		match := commandRE.FindStringSubmatch(strings.TrimSpace(line))
		if len(match) != 2 {
			continue
		}
		fields := strings.Fields(match[1])
		if len(fields) > 0 {
			commands = append(commands, fields)
		}
	}
	return commands
}

func documentedExamplesCheckSelectors(command []string) []string {
	var selectors []string
	for _, token := range command[1:] {
		if strings.HasPrefix(token, "-") {
			continue
		}
		selectors = append(selectors, token)
	}
	return selectors
}

func TestExamplesCommandManifestMatchesPlaygroundRepositoryExamples(t *testing.T) {
	playgroundExamples, err := playgroundRepositoryExamples(playgroundExamplesRoot())
	if err != nil {
		t.Fatalf("load playground repository examples: %v", err)
	}
	cliExamples, err := cliRepositoryExamples()
	if err != nil {
		t.Fatalf("load CLI repository examples: %v", err)
	}

	byID := make(map[string]cliExample, len(cliExamples))
	for _, example := range cliExamples {
		if _, exists := byID[example.ID]; exists {
			t.Fatalf("duplicate CLI example id %s", example.ID)
		}
		byID[example.ID] = example
	}

	for _, playground := range playgroundExamples {
		cli, ok := byID[playground.ID]
		if !ok {
			t.Fatalf("CLI examples list is missing playground repository example %s", playground.ID)
		}
		if cli.Path != playground.Summary {
			t.Fatalf("%s CLI path = %q, playground summary = %q", playground.ID, cli.Path, playground.Summary)
		}
		if playground.Runnable && !cli.Runnable {
			t.Fatalf("%s is runnable in the playground manifest but not in the CLI manifest", playground.ID)
		}
		if !playground.Runnable && !cli.Runnable && strings.TrimSpace(cli.Requires) == "" {
			t.Fatalf("%s is manual/check-only in the CLI manifest but has no requires reason", playground.ID)
		}
	}
}

func TestExamplesCommandChecksSubdirectoryProjectSelector(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{"check", "--timeout=20s", "database/package_managed"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExamplesCommand code = %d, stdout = %q stderr = %q", code, stdout.String(), stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"ok      repo-database-package_managed-main",
		"examples: 1 ok, 0 skipped, 0 failed",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("examples check missing %q\n%s", want, out)
		}
	}
}

func TestExamplesCommandShowAcceptsIDAndPath(t *testing.T) {
	for _, selector := range []string{"repo-hello-counter", "examples/hello/counter.leia", "hello/counter.leia"} {
		t.Run(selector, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := runExamplesCommand([]string{"show", selector}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("runExamplesCommand code = %d, stderr = %q", code, stderr.String())
			}
			out := stdout.String()
			if !strings.Contains(out, "id: repo-hello-counter") || !strings.Contains(out, "makeCounter") {
				t.Fatalf("unexpected show output for %s:\n%s", selector, out)
			}
		})
	}
}

func TestExamplesCommandChecksEmbeddingDocExamples(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{"check", "repo-embedding-go-doc-examples", "repo-embedding-hot_reload_project"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExamplesCommand code = %d, stdout = %q stderr = %q", code, stdout.String(), stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"ok      repo-embedding-go-doc-examples",
		"ok      repo-embedding-hot_reload_project",
		"examples: 2 ok, 0 skipped, 0 failed",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("examples check missing %q\n%s", want, out)
		}
	}
}

func TestExamplesCommandRunsRunnableExample(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{"run", "repo-hello-counter"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExamplesCommand code = %d, stdout = %q stderr = %q", code, stdout.String(), stderr.String())
	}
}

func TestExamplesCommandRefusesManualExample(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{"run", "repo-llm-glm_smoke"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("manual example unexpectedly ran, stdout = %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "manual example") || !strings.Contains(stderr.String(), "LLM provider") {
		t.Fatalf("manual example error missing explanation: %q", stderr.String())
	}
}

func TestExamplesCommandChecksSelectedExamples(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{"check", "--jobs=2", "repo-hello-counter", "repo-llm-agent"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExamplesCommand code = %d, stdout = %q stderr = %q", code, stdout.String(), stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"ok      repo-hello-counter",
		"ok      repo-llm-agent",
		"examples: 2 ok, 0 skipped, 0 failed",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("examples check missing %q\n%s", want, out)
		}
	}
}

func TestExamplesCommandChecksMockFriendlyLLMExamples(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{
		"check",
		"--jobs=4",
		"--timeout=10s",
		"repo-llm-agent",
		"repo-llm-agent_as_tool",
		"repo-llm-direct_turn",
		"repo-llm-incident_response",
		"repo-llm-manual_tool_history",
		"repo-llm-prompt_tagged_messages",
		"repo-llm-rich_agent_demo",
		"repo-llm-streaming_turn",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExamplesCommand code = %d, stdout = %q stderr = %q", code, stdout.String(), stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"ok      repo-llm-agent",
		"ok      repo-llm-agent_as_tool",
		"ok      repo-llm-direct_turn",
		"ok      repo-llm-incident_response",
		"ok      repo-llm-manual_tool_history",
		"ok      repo-llm-prompt_tagged_messages",
		"ok      repo-llm-rich_agent_demo",
		"ok      repo-llm-streaming_turn",
		"examples: 8 ok, 0 skipped, 0 failed",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("examples check missing %q\n%s", want, out)
		}
	}
}

func TestExamplesCommandDefaultCheckSkipsOnlyOptInExamples(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{"check", "--json", "--jobs=6", "--timeout=30s"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExamplesCommand code = %d, stderr = %q", code, stderr.String())
	}
	var payload struct {
		SchemaVersion int                     `json:"schema_version"`
		OK            bool                    `json:"ok"`
		Status        string                  `json:"status"`
		ResultCount   int                     `json:"result_count"`
		Skipped       int                     `json:"skipped"`
		Failed        int                     `json:"failed"`
		Results       []cliExampleCheckResult `json:"results"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("invalid examples check JSON: %v\n%s", err, stdout.String())
	}
	if payload.SchemaVersion != 1 || !payload.OK || payload.Status != "pass" || payload.ResultCount != len(payload.Results) || payload.Failed != 0 {
		t.Fatalf("unexpected examples check payload: %#v", payload)
	}
	allowed := map[string]string{
		"repo-game_engine-chess":                "game/window host access",
		"repo-game_engine-chess_ai":             "game/window host access",
		"repo-game_engine-chess_bench":          "higher playground step budget",
		"repo-game_engine-chess_bench_parallel": "higher playground step budget",
		"repo-game_engine-game":                 "game/window host access",
		"repo-game_engine-tetris":               "game/window host access",
		"repo-llm-glm_direct_agent_tools":       "LLM provider",
		"repo-llm-glm_smoke":                    "LLM provider",
	}
	seen := map[string]bool{}
	for _, result := range payload.Results {
		if result.Status != "skipped" {
			continue
		}
		if result.Requires == "optional q extension" {
			continue
		}
		wantReason, ok := allowed[result.ID]
		if !ok {
			t.Fatalf("example %s is unexpectedly skipped: %s", result.ID, result.Requires)
		}
		if result.Requires != wantReason {
			t.Fatalf("example %s skip reason = %q, want %q", result.ID, result.Requires, wantReason)
		}
		seen[result.ID] = true
	}
	if len(seen) != len(allowed) || payload.Skipped < len(allowed) {
		t.Fatalf("skipped examples = %v payload skipped=%d, want at least %d opt-in examples plus optional extensions", seen, payload.Skipped, len(allowed))
	}
}

func TestExamplesCommandChecksDeterministicSpecialRunners(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{
		"check",
		"--jobs=4",
		"repo-evaluate-basic_assert",
		"repo-evaluate-llm_replay",
		"repo-evaluate-agent_replay",
		"repo-evaluate-judge_replay",
		"repo-evaluate-corpus_metrics",
		"repo-evaluate-multiturn_replay",
		"repo-evaluate-project_agent_regression",
		"repo-ai-coding_agent_replay",
		"repo-ai-coding_agent_project-main",
		"repo-ai-tagged_agent_workflow",
		"repo-ai-general_agent_workflow",
		"repo-ai-general_analysis_assistant",
		"repo-ai-translation_research_assistant",
		"repo-ai-record_replay_trace_project",
		"repo-workflow-support_triage_replay",
		"repo-testing-jsonl_workflow_test",
		"repo-tooling-release_evidence_pipeline",
		"repo-performance-execution_modes_matrix",
		"repo-ui-package_managed-main",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExamplesCommand code = %d, stdout = %q stderr = %q", code, stdout.String(), stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"ok      repo-evaluate-basic_assert",
		"ok      repo-evaluate-llm_replay",
		"ok      repo-evaluate-agent_replay",
		"ok      repo-evaluate-judge_replay",
		"ok      repo-evaluate-corpus_metrics",
		"ok      repo-evaluate-multiturn_replay",
		"ok      repo-evaluate-project_agent_regression",
		"ok      repo-ai-coding_agent_replay",
		"ok      repo-ai-coding_agent_project-main",
		"ok      repo-ai-tagged_agent_workflow",
		"ok      repo-ai-general_agent_workflow",
		"ok      repo-ai-general_analysis_assistant",
		"ok      repo-ai-translation_research_assistant",
		"ok      repo-ai-record_replay_trace_project",
		"ok      repo-workflow-support_triage_replay",
		"ok      repo-testing-jsonl_workflow_test",
		"ok      repo-tooling-release_evidence_pipeline",
		"ok      repo-performance-execution_modes_matrix",
		"ok      repo-ui-package_managed-main",
		"examples: 19 ok, 0 skipped, 0 failed",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("examples check missing %q\n%s", want, out)
		}
	}
}

func TestExamplesCommandRunsEvaluateReplayExample(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{"run", "repo-evaluate-agent_replay"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExamplesCommand code = %d, stdout = %q stderr = %q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), `"status": "ok"`) {
		t.Fatalf("evaluate replay run output = %q, want JSON ok report", stdout.String())
	}
}

func TestExamplesCommandKeepsPackageManifestExamplesNonRunnable(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{"run", "repo-ui-package_managed-main"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("package manifest example unexpectedly ran, stdout = %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "manual example") || !strings.Contains(stderr.String(), "package manifest check") {
		t.Fatalf("manual package example error missing explanation: %q", stderr.String())
	}
}

func TestExamplesCommandChecksDeterministicHostExamples(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{
		"check",
		"--jobs=4",
		"--timeout=10s",
		"repo-web-hello_server",
		"repo-web-fullstack_project-main",
		"repo-web-route_workbench",
		"repo-web-serve_dialect_app",
		"repo-web-tiny_app",
		"repo-web-tiny_fullstack_app",
		"repo-web-webserver",
		"repo-concurrency-context_process",
		"repo-concurrency-goroutine_errors",
		"repo-dialects-shell_filesystem",
		"repo-data_processing-data_oriented-particle_integration",
		"repo-game_engine-game_of_life",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExamplesCommand code = %d, stdout = %q stderr = %q", code, stdout.String(), stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"ok      repo-web-hello_server",
		"ok      repo-web-fullstack_project-main",
		"ok      repo-web-route_workbench",
		"ok      repo-web-serve_dialect_app",
		"ok      repo-web-tiny_app",
		"ok      repo-web-tiny_fullstack_app",
		"ok      repo-web-webserver",
		"ok      repo-concurrency-context_process",
		"ok      repo-concurrency-goroutine_errors",
		"ok      repo-dialects-shell_filesystem",
		"ok      repo-data_processing-data_oriented-particle_integration",
		"ok      repo-game_engine-game_of_life",
		"examples: 12 ok, 0 skipped, 0 failed",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("examples check missing %q\n%s", want, out)
		}
	}
}

func TestExamplesCommandChecksConcurrencyContractExamples(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{
		"check",
		"--jobs=1",
		"--timeout=10s",
		"repo-concurrency-goroutines_channels",
		"repo-concurrency-select_timeout",
		"repo-concurrency-select_default",
		"repo-concurrency-sync_group",
		"repo-concurrency-context_sleep",
		"repo-concurrency-context_cancel",
		"repo-concurrency-sync_group_cancel",
		"repo-concurrency-pipeline_project-main",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExamplesCommand code = %d, stdout = %q stderr = %q", code, stdout.String(), stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"ok      repo-concurrency-goroutines_channels",
		"ok      repo-concurrency-select_timeout",
		"ok      repo-concurrency-select_default",
		"ok      repo-concurrency-sync_group",
		"ok      repo-concurrency-context_sleep",
		"ok      repo-concurrency-context_cancel",
		"ok      repo-concurrency-sync_group_cancel",
		"ok      repo-concurrency-pipeline_project-main",
		"examples: 8 ok, 0 skipped, 0 failed",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("examples check missing %q\n%s", want, out)
		}
	}
}

func TestExamplesCommandChecksReadmeCapabilityEvidenceExamples(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{
		"check",
		"--jobs=2",
		"--timeout=20s",
		"repo-embedding-go-doc-examples",
		"repo-embedding-hot_reload_project",
		"repo-llm-agent",
		"repo-ai-coding_agent_replay",
		"repo-ai-coding_agent_project-main",
		"repo-evaluate-agent_replay",
		"repo-automation-invoice_reconciliation",
		"repo-automation-release_fixture_matrix",
		"repo-automation-release_risk_digest",
		"repo-dialects-shell_filesystem",
		"repo-dialects-sql_result_analytics",
		"repo-concurrency-goroutines_channels",
		"repo-concurrency-select_timeout",
		"repo-concurrency-sync_group",
		"repo-concurrency-pipeline_project-main",
		"repo-data_processing-data_oriented-soa_kernels",
		"repo-data_processing-data_oriented-dense_matrix_vec_kernels",
		"repo-database-package_managed-main",
		"repo-web-serve_dialect_app",
		"repo-web-tiny_fullstack_app",
		"repo-site-static_docs_generator",
		"repo-site-release_dashboard",
		"repo-tooling-package_manager_workflow-main",
		"repo-tooling-package_manager_workflow-local-metadata-report",
		"repo-macos-package_managed-main",
		"repo-macos-package_managed-adapter-automation",
		"repo-performance-execution_modes_matrix",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExamplesCommand code = %d, stdout = %q stderr = %q", code, stdout.String(), stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"ok      repo-embedding-go-doc-examples",
		"ok      repo-embedding-hot_reload_project",
		"ok      repo-llm-agent",
		"ok      repo-ai-coding_agent_replay",
		"ok      repo-ai-coding_agent_project-main",
		"ok      repo-evaluate-agent_replay",
		"ok      repo-automation-invoice_reconciliation",
		"ok      repo-automation-release_fixture_matrix",
		"ok      repo-automation-release_risk_digest",
		"ok      repo-dialects-shell_filesystem",
		"ok      repo-dialects-sql_result_analytics",
		"ok      repo-concurrency-goroutines_channels",
		"ok      repo-concurrency-select_timeout",
		"ok      repo-concurrency-sync_group",
		"ok      repo-concurrency-pipeline_project-main",
		"ok      repo-data_processing-data_oriented-soa_kernels",
		"ok      repo-data_processing-data_oriented-dense_matrix_vec_kernels",
		"ok      repo-database-package_managed-main",
		"ok      repo-web-serve_dialect_app",
		"ok      repo-web-tiny_fullstack_app",
		"ok      repo-site-static_docs_generator",
		"ok      repo-site-release_dashboard",
		"ok      repo-tooling-package_manager_workflow-main",
		"ok      repo-tooling-package_manager_workflow-local-metadata-report",
		"ok      repo-macos-package_managed-main",
		"ok      repo-macos-package_managed-adapter-automation",
		"ok      repo-performance-execution_modes_matrix",
		"examples: 27 ok, 0 skipped, 0 failed",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("README capability examples check missing %q\n%s", want, out)
		}
	}
}

func TestExamplesCommandChecksSelectedExamplesJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{"check", "--json", "repo-hello-counter", "repo-llm-agent"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runExamplesCommand code = %d, stderr = %q", code, stderr.String())
	}
	var payload struct {
		SchemaVersion int                     `json:"schema_version"`
		OK            bool                    `json:"ok"`
		Status        string                  `json:"status"`
		ResultCount   int                     `json:"result_count"`
		Runnable      int                     `json:"runnable"`
		Skipped       int                     `json:"skipped"`
		Failed        int                     `json:"failed"`
		Results       []cliExampleCheckResult `json:"results"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("invalid examples check JSON: %v\n%s", err, stdout.String())
	}
	if payload.SchemaVersion != 1 || !payload.OK || payload.Status != "pass" || payload.ResultCount != len(payload.Results) || payload.Runnable != 2 || payload.Skipped != 0 || payload.Failed != 0 {
		t.Fatalf("unexpected examples check payload: %#v", payload)
	}
}

func TestExamplesCommandCheckRejectsInvalidTimeout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runExamplesCommand([]string{"check", "--timeout=0s", "repo-hello-counter"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runExamplesCommand code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--timeout must be positive") {
		t.Fatalf("stderr = %q, want timeout validation", stderr.String())
	}
}
