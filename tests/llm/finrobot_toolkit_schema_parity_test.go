package leia_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type toolkitParityLedger struct {
	SchemaVersion     int    `json:"schema_version"`
	ID                string `json:"id"`
	Scope             string `json:"scope"`
	NotFinRobotSyntax bool   `json:"not_finrobot_syntax"`
	Source            struct {
		SourcePath        string   `json:"source_path"`
		SourceFunctions   []string `json:"source_functions"`
		DependencyMarkers []string `json:"source_dependency_markers"`
	} `json:"source"`
	Target struct {
		BaselineBranch              string `json:"baseline_branch"`
		ProviderFreeDefault         bool   `json:"provider_free_default"`
		LiveNetworkDefault          bool   `json:"live_network_default"`
		RealDependencyImportDefault bool   `json:"real_dependency_import_default"`
		RuntimeExecutionDefault     bool   `json:"runtime_execution_default"`
		LedgerDirectory             string `json:"ledger_directory"`
	} `json:"target"`
	GlobalGates  []string `json:"global_gates"`
	Capabilities []struct {
		ID              string   `json:"id"`
		SourceConcepts  []string `json:"source_concepts"`
		DialectContract string   `json:"dialect_contract"`
		ProviderFree    bool     `json:"provider_free"`
		SchemaRef       string   `json:"schema_ref"`
	} `json:"capabilities"`
	SourceMappings []struct {
		SourceFunction            string   `json:"source_function"`
		NormalizedDialectName     string   `json:"normalized_dialect_name"`
		CoveredCapabilities       []string `json:"covered_capabilities"`
		ParityNotes               []string `json:"parity_notes"`
		DefaultEnabled            bool     `json:"default_enabled"`
		RequiresOptInCapabilities []string `json:"requires_opt_in_capabilities"`
	} `json:"source_mappings"`
	CallerExecutorContract struct {
		RoleNames                []string `json:"role_names"`
		CallerResponsibilities   []string `json:"caller_responsibilities"`
		ExecutorResponsibilities []string `json:"executor_responsibilities"`
		RegistrationEdgeFields   []string `json:"registration_edge_fields"`
	} `json:"caller_executor_contract"`
	SchemaContract struct {
		InputSchemaRef             string   `json:"input_schema_ref"`
		ResultSchemaRef            string   `json:"result_schema_ref"`
		SchemaLanguage             string   `json:"schema_language"`
		ProviderWireFormat         string   `json:"provider_wire_format"`
		RequiredToolFields         []string `json:"required_tool_fields"`
		AdditionalPropertiesPolicy string   `json:"additional_properties_policy"`
	} `json:"schema_contract"`
	ResultEnvelopeContract struct {
		SuccessFixture        string   `json:"success_fixture"`
		ErrorFixture          string   `json:"error_fixture"`
		SuccessRequired       []string `json:"success_required"`
		ErrorRequired         []string `json:"error_required"`
		StructuredErrorFields []string `json:"structured_error_fields"`
	} `json:"result_envelope_contract"`
	OutputAdapters []struct {
		ID                         string `json:"id"`
		SourceBehavior             string `json:"source_behavior"`
		DialectOutputFormat        string `json:"dialect_output_format"`
		PreserveRawType            bool   `json:"preserve_raw_type"`
		TextRenderingRequired      bool   `json:"text_rendering_required"`
		StructuredRowsRequired     bool   `json:"structured_rows_required"`
		ProviderFree               bool   `json:"provider_free"`
		ForbiddenRuntimeDependency string `json:"forbidden_runtime_dependency"`
	} `json:"output_adapters"`
	NegativeGates struct {
		ForbiddenImports       []string `json:"forbidden_imports"`
		ForbiddenLiveBehaviors []string `json:"forbidden_live_behaviors"`
		ForbiddenClaims        []string `json:"forbidden_claims"`
	} `json:"negative_gates"`
}

type toolkitParitySchema struct {
	SchemaVersion         int      `json:"schema_version"`
	ID                    string   `json:"id"`
	ProviderFree          bool     `json:"provider_free"`
	Domain                string   `json:"domain"`
	Required              []string `json:"required"`
	CanonicalFields       []string `json:"canonical_fields"`
	SuccessRequired       []string `json:"success_required"`
	ErrorRequired         []string `json:"error_required"`
	StructuredErrorFields []string `json:"structured_error_fields"`
	AdditionalProperties  bool     `json:"additional_properties"`
	MissingRequiredPolicy string   `json:"missing_required_policy"`
}

func TestFinRobotToolkitParityLedgerIsGenericProviderFreeToolDialect(t *testing.T) {
	base := toolkitParityDir(t)
	ledger := loadToolkitParityLedger(t, base)

	if ledger.SchemaVersion != 1 || ledger.ID != "finrobot-toolkit-schema-parity-ledger" {
		t.Fatalf("ledger header = schema %d id %q", ledger.SchemaVersion, ledger.ID)
	}
	if ledger.Scope != "generic_ai_tool_dialect" || !ledger.NotFinRobotSyntax {
		t.Fatalf("ledger must be generic AI tool dialect, not FinRobot syntax: scope=%q flag=%v", ledger.Scope, ledger.NotFinRobotSyntax)
	}
	if ledger.Source.SourcePath != "finrobot/toolkits.py" {
		t.Fatalf("source path = %q", ledger.Source.SourcePath)
	}
	for _, want := range []string{"stringify_output", "register_toolkits", "register_code_writing", "register_tookits_from_cls"} {
		if !stringSliceContains(ledger.Source.SourceFunctions, want) {
			t.Fatalf("source function %q missing: %#v", want, ledger.Source.SourceFunctions)
		}
	}
	if ledger.Target.BaselineBranch != "origin/codex/ai-dialect-polish" ||
		!ledger.Target.ProviderFreeDefault ||
		ledger.Target.LiveNetworkDefault ||
		ledger.Target.RealDependencyImportDefault ||
		ledger.Target.RuntimeExecutionDefault {
		t.Fatalf("provider-free target defaults are wrong: %#v", ledger.Target)
	}
	if ledger.Target.LedgerDirectory != "examples/ai/finrobot_translation/toolkit_parity" {
		t.Fatalf("ledger directory = %q", ledger.Target.LedgerDirectory)
	}

	joinedGates := strings.ToLower(strings.Join(ledger.GlobalGates, " "))
	for _, want := range []string{"generic ai tool dialect", "not finrobot syntax", "caller", "executor", "structured error", "string", "table"} {
		if !strings.Contains(joinedGates, want) {
			t.Fatalf("global gates missing %q evidence: %s", want, joinedGates)
		}
	}
	for _, blocked := range []string{"autogen", "pandas", "q/runtime"} {
		if !stringSliceContains(ledger.NegativeGates.ForbiddenImports, blocked) {
			t.Fatalf("forbidden imports missing %q: %#v", blocked, ledger.NegativeGates.ForbiddenImports)
		}
	}
	for _, blocked := range []string{"network", "provider_api_call", "local_code_execution", "finance_vendor_call"} {
		if !stringSliceContains(ledger.NegativeGates.ForbiddenLiveBehaviors, blocked) {
			t.Fatalf("forbidden live behavior missing %q: %#v", blocked, ledger.NegativeGates.ForbiddenLiveBehaviors)
		}
	}
}

func TestFinRobotToolkitParityCoversRegistrationSchemaAndCallerExecutor(t *testing.T) {
	base := toolkitParityDir(t)
	ledger := loadToolkitParityLedger(t, base)

	wantCapabilities := []string{
		"ai.tool.registry.from_callable",
		"ai.tool.registry.from_class",
		"ai.tool.caller_executor.split",
		"ai.tool.schema.declarative",
		"ai.tool.result.envelope",
		"ai.tool.error.structured",
		"ai.tool.output.adapter.string",
		"ai.tool.output.adapter.table",
	}
	capabilities := map[string]bool{}
	for _, capability := range ledger.Capabilities {
		if capability.ID == "" || capability.DialectContract == "" || !capability.ProviderFree || capability.SchemaRef == "" {
			t.Fatalf("incomplete capability: %#v", capability)
		}
		if len(capability.SourceConcepts) == 0 {
			t.Fatalf("capability %q has no source concepts", capability.ID)
		}
		if _, err := os.Stat(filepath.Join(base, filepath.FromSlash(capability.SchemaRef))); err != nil {
			t.Fatalf("schema ref for %s: %v", capability.ID, err)
		}
		capabilities[capability.ID] = true
	}
	for _, want := range wantCapabilities {
		if !capabilities[want] {
			t.Fatalf("capability %q missing from ledger", want)
		}
	}

	mappings := map[string][]string{}
	for _, mapping := range ledger.SourceMappings {
		if mapping.SourceFunction == "" || mapping.NormalizedDialectName == "" || len(mapping.CoveredCapabilities) == 0 {
			t.Fatalf("incomplete source mapping: %#v", mapping)
		}
		for _, capability := range mapping.CoveredCapabilities {
			if !capabilities[capability] {
				t.Fatalf("mapping %s references unknown capability %q", mapping.SourceFunction, capability)
			}
		}
		mappings[mapping.SourceFunction] = mapping.CoveredCapabilities
	}
	for _, sourceFunction := range ledger.Source.SourceFunctions {
		if len(mappings[sourceFunction]) == 0 {
			t.Fatalf("source function %q has no parity mapping", sourceFunction)
		}
	}
	if !stringSliceContains(mappings["register_toolkits"], "ai.tool.error.structured") {
		t.Fatalf("register_toolkits must cover structured registration errors: %#v", mappings["register_toolkits"])
	}
	if !stringSliceContains(mappings["register_tookits_from_cls"], "ai.tool.registry.from_class") {
		t.Fatalf("register_tookits_from_cls must cover class toolkit expansion: %#v", mappings["register_tookits_from_cls"])
	}

	if strings.Join(ledger.CallerExecutorContract.RoleNames, ",") != "caller,executor" {
		t.Fatalf("caller/executor roles = %#v", ledger.CallerExecutorContract.RoleNames)
	}
	for _, want := range []string{"tool_name", "caller_id", "executor_id", "capability_ids", "schema_ref", "output_adapter"} {
		if !stringSliceContains(ledger.CallerExecutorContract.RegistrationEdgeFields, want) {
			t.Fatalf("registration edge fields missing %q: %#v", want, ledger.CallerExecutorContract.RegistrationEdgeFields)
		}
	}
	if len(ledger.CallerExecutorContract.CallerResponsibilities) == 0 || len(ledger.CallerExecutorContract.ExecutorResponsibilities) == 0 {
		t.Fatalf("caller/executor responsibilities incomplete: %#v", ledger.CallerExecutorContract)
	}
}

func TestFinRobotToolkitParitySchemasResultsErrorsAndAdapters(t *testing.T) {
	base := toolkitParityDir(t)
	ledger := loadToolkitParityLedger(t, base)

	if ledger.SchemaContract.ProviderWireFormat != "none" ||
		ledger.SchemaContract.SchemaLanguage != "json_schema_compatible_metadata" ||
		ledger.SchemaContract.AdditionalPropertiesPolicy != "reject_unknown_tool_metadata" {
		t.Fatalf("schema contract is provider-specific or too loose: %#v", ledger.SchemaContract)
	}
	for _, want := range []string{"tool_name", "description", "capability_ids", "input_schema", "output_adapter"} {
		if !stringSliceContains(ledger.SchemaContract.RequiredToolFields, want) {
			t.Fatalf("required tool field %q missing: %#v", want, ledger.SchemaContract.RequiredToolFields)
		}
	}

	for _, rel := range []string{ledger.SchemaContract.InputSchemaRef, ledger.SchemaContract.ResultSchemaRef} {
		var schema toolkitParitySchema
		decodeToolkitParityJSONFile(t, filepath.Join(base, filepath.FromSlash(rel)), &schema)
		if schema.SchemaVersion != 1 || schema.ID == "" || !schema.ProviderFree || schema.Domain != "generic_ai_tool_dialect" {
			t.Fatalf("schema header invalid for %s: %#v", rel, schema)
		}
		if schema.AdditionalProperties {
			t.Fatalf("schema %s must reject additional properties", rel)
		}
		if schema.MissingRequiredPolicy == "" || len(schema.Required) == 0 || len(schema.CanonicalFields) == 0 {
			t.Fatalf("schema %s is missing policies or fields: %#v", rel, schema)
		}
	}

	for _, want := range []string{"ok", "tool_name", "output_format", "content", "provenance"} {
		if !stringSliceContains(ledger.ResultEnvelopeContract.SuccessRequired, want) {
			t.Fatalf("success envelope required field missing %q: %#v", want, ledger.ResultEnvelopeContract.SuccessRequired)
		}
	}
	for _, want := range []string{"code", "message", "stage", "retryable", "details"} {
		if !stringSliceContains(ledger.ResultEnvelopeContract.StructuredErrorFields, want) {
			t.Fatalf("structured error field missing %q: %#v", want, ledger.ResultEnvelopeContract.StructuredErrorFields)
		}
	}

	var success struct {
		OK           bool           `json:"ok"`
		ToolName     string         `json:"tool_name"`
		OutputFormat string         `json:"output_format"`
		Content      string         `json:"content"`
		Table        map[string]any `json:"table"`
		Provenance   map[string]any `json:"provenance"`
	}
	decodeToolkitParityJSONFile(t, filepath.Join(base, filepath.FromSlash(ledger.ResultEnvelopeContract.SuccessFixture)), &success)
	if !success.OK || success.ToolName == "" || success.OutputFormat != "table" || success.Content == "" || len(success.Table) == 0 || success.Provenance["provider_free"] != true {
		t.Fatalf("success fixture does not prove table result envelope: %#v", success)
	}

	var failure struct {
		OK       bool   `json:"ok"`
		ToolName string `json:"tool_name"`
		Error    struct {
			Code      string         `json:"code"`
			Message   string         `json:"message"`
			Stage     string         `json:"stage"`
			Retryable bool           `json:"retryable"`
			Details   map[string]any `json:"details"`
		} `json:"error"`
	}
	decodeToolkitParityJSONFile(t, filepath.Join(base, filepath.FromSlash(ledger.ResultEnvelopeContract.ErrorFixture)), &failure)
	if failure.OK || failure.ToolName == "" || failure.Error.Code == "" || failure.Error.Stage != "registration" || failure.Error.Retryable {
		t.Fatalf("error fixture does not prove structured registration error: %#v", failure)
	}

	adapters := map[string]bool{}
	for _, adapter := range ledger.OutputAdapters {
		if adapter.ID == "" || adapter.DialectOutputFormat == "" || !adapter.ProviderFree {
			t.Fatalf("incomplete output adapter: %#v", adapter)
		}
		adapters[adapter.ID] = true
		if adapter.ID == "table" && (!adapter.TextRenderingRequired || !adapter.StructuredRowsRequired || adapter.ForbiddenRuntimeDependency != "pandas") {
			t.Fatalf("table adapter must keep text plus structured rows without Pandas dependency: %#v", adapter)
		}
		if adapter.ID == "string" && (!adapter.PreserveRawType || adapter.DialectOutputFormat != "text") {
			t.Fatalf("string adapter must preserve raw type metadata and emit text: %#v", adapter)
		}
	}
	for _, want := range []string{"string", "table"} {
		if !adapters[want] {
			t.Fatalf("output adapter %q missing", want)
		}
	}
}

func TestFinRobotToolkitParityFilesStayProviderFree(t *testing.T) {
	base := toolkitParityDir(t)
	ledger := loadToolkitParityLedger(t, base)
	files := []string{
		filepath.Join(base, "README.md"),
		filepath.Join(base, "ledger.json"),
		filepath.Join(base, filepath.FromSlash(ledger.SchemaContract.InputSchemaRef)),
		filepath.Join(base, filepath.FromSlash(ledger.SchemaContract.ResultSchemaRef)),
		filepath.Join(base, filepath.FromSlash(ledger.ResultEnvelopeContract.SuccessFixture)),
		filepath.Join(base, filepath.FromSlash(ledger.ResultEnvelopeContract.ErrorFixture)),
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

func toolkitParityDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "toolkit_parity")
}

func loadToolkitParityLedger(t *testing.T, base string) toolkitParityLedger {
	t.Helper()
	var ledger toolkitParityLedger
	decodeToolkitParityJSONFile(t, filepath.Join(base, "ledger.json"), &ledger)
	return ledger
}

func decodeToolkitParityJSONFile(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
