package leia_test

import (
	"path/filepath"
	"testing"
)

func TestFinRobotOpenBBOptionalIntegrationProviderFreeGate(t *testing.T) {
	base := optionalIntegrationsLivePackageDir(t)
	var contract struct {
		ProviderFree          bool `json:"provider_free"`
		LiveNetwork           bool `json:"live_network"`
		RealDependencyImports bool `json:"real_dependency_imports"`
		NoLiveImportDefault   bool `json:"no_live_import_default"`
		Gates                 []struct {
			ID                          string         `json:"id"`
			Capability                  string         `json:"capability"`
			FixtureKey                  string         `json:"fixture_key"`
			CleanSkip                   bool           `json:"clean_skip"`
			RequiresCredentials         bool           `json:"requires_credentials"`
			LiveNetwork                 bool           `json:"live_network"`
			DependencyImported          bool           `json:"dependency_imported"`
			StatusWithoutDependency     string         `json:"status_without_dependency"`
			ProviderCredentialsRequired bool           `json:"provider_credentials_required"`
			CredentialAbsentSafe        bool           `json:"credential_absent_safe"`
			NoLiveImportDefault         bool           `json:"no_live_import_default"`
			RequestSchema               map[string]any `json:"request_schema"`
			ResultEnvelope              map[string]any `json:"tool_adapter_result_envelope"`
			AbsenceGates                map[string]struct {
				Env          string `json:"env"`
				CleanSkip    bool   `json:"clean_skip"`
				AbsentStatus string `json:"absent_status"`
			} `json:"absence_gates"`
			TermsMetadata map[string]any `json:"terms_metadata"`
		} `json:"gates"`
	}
	decodeOptionalLiveJSONFile(t, filepath.Join(base, "contracts", "optional_integration_capability_gates.json"), &contract)
	if !contract.ProviderFree || contract.LiveNetwork || contract.RealDependencyImports || !contract.NoLiveImportDefault {
		t.Fatalf("OpenBB contract defaults must be provider-free and no-live-import: %#v", contract)
	}

	var openbb *struct {
		ID                          string         `json:"id"`
		Capability                  string         `json:"capability"`
		FixtureKey                  string         `json:"fixture_key"`
		CleanSkip                   bool           `json:"clean_skip"`
		RequiresCredentials         bool           `json:"requires_credentials"`
		LiveNetwork                 bool           `json:"live_network"`
		DependencyImported          bool           `json:"dependency_imported"`
		StatusWithoutDependency     string         `json:"status_without_dependency"`
		ProviderCredentialsRequired bool           `json:"provider_credentials_required"`
		CredentialAbsentSafe        bool           `json:"credential_absent_safe"`
		NoLiveImportDefault         bool           `json:"no_live_import_default"`
		RequestSchema               map[string]any `json:"request_schema"`
		ResultEnvelope              map[string]any `json:"tool_adapter_result_envelope"`
		AbsenceGates                map[string]struct {
			Env          string `json:"env"`
			CleanSkip    bool   `json:"clean_skip"`
			AbsentStatus string `json:"absent_status"`
		} `json:"absence_gates"`
		TermsMetadata map[string]any `json:"terms_metadata"`
	}
	for i := range contract.Gates {
		if contract.Gates[i].ID == "openbb" {
			openbb = &contract.Gates[i]
			break
		}
	}
	if openbb == nil {
		t.Fatal("missing openbb gate")
	}
	if openbb.Capability != "optional.data.openbb.market_data" || openbb.FixtureKey != "openbb:market_data:ACME:offline" {
		t.Fatalf("OpenBB identity = capability %q fixture %q", openbb.Capability, openbb.FixtureKey)
	}
	if !openbb.CleanSkip || openbb.RequiresCredentials || openbb.LiveNetwork || openbb.DependencyImported ||
		openbb.StatusWithoutDependency != "skipped" || openbb.ProviderCredentialsRequired ||
		!openbb.CredentialAbsentSafe || !openbb.NoLiveImportDefault {
		t.Fatalf("OpenBB gate must skip cleanly without credentials, imports, or network: %#v", openbb)
	}

	assertOpenBBRequestSchema(t, openbb.RequestSchema)
	assertOpenBBResultEnvelope(t, openbb.ResultEnvelope)
	assertOpenBBAbsenceGate(t, openbb.AbsenceGates, "pat", "OPENBB_PAT")
	assertOpenBBAbsenceGate(t, openbb.AbsenceGates, "database", "OPENBB_DATABASE_URL")
	if openbb.TermsMetadata["mode"] != "fixture_replay" ||
		openbb.TermsMetadata["metadata_only"] != true ||
		openbb.TermsMetadata["live_terms_required_before_network_use"] != true ||
		openbb.TermsMetadata["live_network_enabled"] != false {
		t.Fatalf("OpenBB terms metadata must stay fixture-only and explicit: %#v", openbb.TermsMetadata)
	}
}

func TestFinRobotOpenBBOptionalIntegrationFixtureEnvelope(t *testing.T) {
	base := optionalIntegrationsLivePackageDir(t)
	var fixtures struct {
		Fixtures []struct {
			FixtureKey    string         `json:"fixture_key"`
			IntegrationID string         `json:"integration_id"`
			Capability    string         `json:"capability"`
			Metadata      map[string]any `json:"metadata"`
		} `json:"fixtures"`
	}
	decodeOptionalLiveJSONFile(t, filepath.Join(base, "fixtures", "provider_free_fixture_index.json"), &fixtures)

	var metadata map[string]any
	for _, fixture := range fixtures.Fixtures {
		if fixture.IntegrationID == "openbb" {
			if fixture.FixtureKey != "openbb:market_data:ACME:offline" || fixture.Capability != "optional.data.openbb.market_data" {
				t.Fatalf("OpenBB fixture identity mismatch: %#v", fixture)
			}
			metadata = fixture.Metadata
			break
		}
	}
	if metadata == nil {
		t.Fatal("missing OpenBB fixture metadata")
	}
	request, ok := metadata["request"].(map[string]any)
	if !ok || request["dataset"] != "market_data" {
		t.Fatalf("OpenBB fixture request missing dataset/query schema: %#v", metadata["request"])
	}
	query, ok := request["query"].(map[string]any)
	if !ok || query["symbol"] != "ACME" || query["asset_class"] != "equity" || query["interval"] != "1d" || query["adjusted"] != true {
		t.Fatalf("OpenBB fixture query mismatch: %#v", request["query"])
	}
	envelope, ok := metadata["result_envelope"].(map[string]any)
	if !ok {
		t.Fatalf("OpenBB fixture result envelope missing: %#v", metadata["result_envelope"])
	}
	if envelope["status"] != "ok" || envelope["capability"] != "optional.data.openbb.market_data" || envelope["fixture_key"] != "openbb:market_data:ACME:offline" {
		t.Fatalf("OpenBB envelope identity mismatch: %#v", envelope)
	}
	envelopeMetadata, ok := envelope["metadata"].(map[string]any)
	if !ok || envelopeMetadata["provider_free"] != true || envelopeMetadata["live_network"] != false || envelopeMetadata["real_dependency_imports"] != false {
		t.Fatalf("OpenBB envelope metadata must be provider-free fixture replay: %#v", envelope["metadata"])
	}
	absence, ok := metadata["absence_gates"].(map[string]any)
	if !ok || absence["OPENBB_PAT"] != "skipped" || absence["OPENBB_DATABASE_URL"] != "skipped" {
		t.Fatalf("OpenBB absent PAT/database gates must cleanly skip: %#v", metadata["absence_gates"])
	}
	terms, ok := metadata["terms"].(map[string]any)
	if !ok || terms["mode"] != "fixture_replay" || terms["metadata_only"] != true || terms["live_terms_required_before_network_use"] != true {
		t.Fatalf("OpenBB terms fixture metadata mismatch: %#v", metadata["terms"])
	}
}

func assertOpenBBRequestSchema(t *testing.T, schema map[string]any) {
	t.Helper()
	if schema["schema_version"] != float64(1) || schema["type"] != "object" {
		t.Fatalf("OpenBB request schema header mismatch: %#v", schema)
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("OpenBB request schema missing properties: %#v", schema)
	}
	dataset, ok := properties["dataset"].(map[string]any)
	if !ok || dataset["const"] != "market_data" {
		t.Fatalf("OpenBB request dataset schema mismatch: %#v", properties["dataset"])
	}
	query, ok := properties["query"].(map[string]any)
	if !ok || query["type"] != "object" {
		t.Fatalf("OpenBB request query schema mismatch: %#v", properties["query"])
	}
	queryProperties, ok := query["properties"].(map[string]any)
	if !ok {
		t.Fatalf("OpenBB request query properties missing: %#v", query)
	}
	for _, key := range []string{"symbol", "asset_class", "interval", "adjusted"} {
		if queryProperties[key] == nil {
			t.Fatalf("OpenBB request query missing %q schema: %#v", key, queryProperties)
		}
	}
}

func assertOpenBBResultEnvelope(t *testing.T, envelope map[string]any) {
	t.Helper()
	if envelope["provider_free"] != true || envelope["live_network"] != false || envelope["real_dependency_imports"] != false {
		t.Fatalf("OpenBB result envelope must be provider-free: %#v", envelope)
	}
	if envelope["skip_status"] != "skipped" || envelope["data_source"] != "fixture" {
		t.Fatalf("OpenBB result envelope skip/source mismatch: %#v", envelope)
	}
	for _, key := range []string{"status", "capability", "fixture_key", "data", "metadata"} {
		if !openBBStringSliceContains(envelope["required_fields"], key) {
			t.Fatalf("OpenBB result envelope missing required field %q: %#v", key, envelope["required_fields"])
		}
	}
	for _, status := range []string{"ok", "skipped"} {
		if !openBBStringSliceContains(envelope["status_values"], status) {
			t.Fatalf("OpenBB result envelope missing status %q: %#v", status, envelope["status_values"])
		}
	}
}

func assertOpenBBAbsenceGate(t *testing.T, gates map[string]struct {
	Env          string `json:"env"`
	CleanSkip    bool   `json:"clean_skip"`
	AbsentStatus string `json:"absent_status"`
}, key string, env string) {
	t.Helper()
	gate, ok := gates[key]
	if !ok {
		t.Fatalf("OpenBB missing absence gate %q", key)
	}
	if gate.Env != env || !gate.CleanSkip || gate.AbsentStatus != "skipped" {
		t.Fatalf("OpenBB absence gate %q mismatch: %#v", key, gate)
	}
}

func openBBStringSliceContains(value any, want string) bool {
	values, ok := value.([]any)
	if !ok {
		return false
	}
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
