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
)

type financeNormalizersManifest struct {
	SchemaVersion               int                 `json:"schema_version"`
	ID                          string              `json:"id"`
	PackageName                 string              `json:"package_name"`
	ProviderFree                bool                `json:"provider_free"`
	LiveNetworkDefault          bool                `json:"live_network_default"`
	RealDependencyImportDefault bool                `json:"real_dependency_import_default"`
	SourceModules               []string            `json:"source_modules"`
	Entrypoints                 map[string]string   `json:"entrypoints"`
	Schemas                     map[string]string   `json:"schemas"`
	Fixtures                    map[string]string   `json:"fixtures"`
	NormalizerDomains           []string            `json:"normalizer_domains"`
	Capabilities                []string            `json:"capabilities"`
	BlockedImports              []string            `json:"blocked_imports"`
	FieldCanonicalization       map[string]string   `json:"field_canonicalization"`
	StaleMissingPolicy          map[string]any      `json:"stale_missing_policy"`
	DeterministicOrdering       map[string][]string `json:"deterministic_ordering"`
	Credentials                 struct {
		Required          []string `json:"required"`
		Optional          []string `json:"optional"`
		SecretEnvPatterns []string `json:"secret_env_patterns"`
		Policy            string   `json:"policy"`
	} `json:"credentials"`
	DefaultPolicy struct {
		Mode                        string `json:"mode"`
		LiveNetwork                 bool   `json:"live_network"`
		ProviderCredentialsRequired bool   `json:"provider_credentials_required"`
		RealDependencyImports       bool   `json:"real_dependency_imports"`
		CleanSkipWithoutDependency  bool   `json:"clean_skip_without_dependency"`
		FixtureHook                 string `json:"fixture_hook"`
	} `json:"default_policy"`
	ProviderGate struct {
		AllowNetwork         bool     `json:"allow_network"`
		RequiredCredentials  []string `json:"required_credentials"`
		OptionalCredentials  []string `json:"optional_credentials"`
		ProviderSDKsRequired bool     `json:"provider_sdks_required"`
		TestRule             string   `json:"test_rule"`
	} `json:"provider_gate"`
}

type financeNormalizersContract struct {
	Domains                    []string       `json:"domains"`
	CanonicalFieldRules        []string       `json:"canonical_field_rules"`
	StaleMissingPolicy         map[string]any `json:"stale_missing_policy"`
	TypedNormalizationBoundary struct {
		Schema       string   `json:"schema"`
		Covers       []string `json:"covers"`
		ProviderFree bool     `json:"provider_free"`
	} `json:"typed_normalization_boundary"`
	CorporateActionAdjustment struct {
		Schema                   string   `json:"schema"`
		RequiredAdjustmentFields []string `json:"required_adjustment_fields"`
		ProviderFree             bool     `json:"provider_free"`
	} `json:"corporate_action_adjustment"`
	ValidationErrorTaxonomy struct {
		Schema       string   `json:"schema"`
		ErrorCodes   []string `json:"error_codes"`
		ProviderFree bool     `json:"provider_free"`
	} `json:"validation_error_taxonomy"`
}

type financeNormalizerSchema struct {
	SchemaVersion         int      `json:"schema_version"`
	ID                    string   `json:"id"`
	Domain                string   `json:"domain"`
	Required              []string `json:"required"`
	CanonicalFields       []string `json:"canonical_fields"`
	DeterministicOrder    []string `json:"deterministic_order"`
	MissingRequiredPolicy string   `json:"missing_required_policy"`
	MissingOptionalPolicy string   `json:"missing_optional_policy"`
	ProviderFree          bool     `json:"provider_free"`
}

func TestFinRobotFinanceNormalizersLivePackageManifest(t *testing.T) {
	base := financeNormalizersLivePackageDir(t)
	manifest := loadFinanceNormalizersManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.ID != "finrobot-finance-normalizers-live-package" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ID)
	}
	if manifest.PackageName != "leia-finrobot-finance-normalizers" {
		t.Fatalf("package name = %q", manifest.PackageName)
	}
	if !manifest.ProviderFree || manifest.LiveNetworkDefault || manifest.RealDependencyImportDefault {
		t.Fatalf("provider-free defaults = provider_free:%v live_network:%v imports:%v", manifest.ProviderFree, manifest.LiveNetworkDefault, manifest.RealDependencyImportDefault)
	}
	if len(manifest.Credentials.Required) != 0 || len(manifest.Credentials.Optional) != 0 || len(manifest.Credentials.SecretEnvPatterns) != 0 {
		t.Fatalf("normalizer skeleton must not declare credentials: %#v", manifest.Credentials)
	}
	if manifest.DefaultPolicy.Mode != "fixture_replay" ||
		manifest.DefaultPolicy.LiveNetwork ||
		manifest.DefaultPolicy.ProviderCredentialsRequired ||
		manifest.DefaultPolicy.RealDependencyImports ||
		!manifest.DefaultPolicy.CleanSkipWithoutDependency {
		t.Fatalf("default policy must stay fixture-only and clean-skip safe: %#v", manifest.DefaultPolicy)
	}
	if manifest.ProviderGate.AllowNetwork || manifest.ProviderGate.ProviderSDKsRequired ||
		len(manifest.ProviderGate.RequiredCredentials) != 0 ||
		len(manifest.ProviderGate.OptionalCredentials) != 0 {
		t.Fatalf("provider gate must stay closed: %#v", manifest.ProviderGate)
	}

	wantDomains := []string{"statements", "ratios", "market", "news", "sec", "peer", "provenance", "normalization_boundary", "corporate_actions", "validation"}
	if !reflect.DeepEqual(manifest.NormalizerDomains, wantDomains) {
		t.Fatalf("domains = %#v, want %#v", manifest.NormalizerDomains, wantDomains)
	}
	for _, key := range []string{"smoke", "contract", "fixture_index"} {
		if manifest.Entrypoints[key] == "" {
			t.Fatalf("missing entrypoint %q", key)
		}
		assertFinanceNormalizersJSONOrLeiaFile(t, filepath.Join(base, manifest.Entrypoints[key]))
	}
	for _, key := range []string{"statement", "ratio", "market", "news", "sec", "peer", "provenance", "policy", "normalization_boundary", "corporate_action", "validation_error"} {
		if manifest.Schemas[key] == "" {
			t.Fatalf("missing schema %q", key)
		}
		assertFinanceNormalizersJSONFile(t, filepath.Join(base, manifest.Schemas[key]))
		if manifest.Fixtures[key] == "" {
			t.Fatalf("missing fixture %q", key)
		}
		assertFinanceNormalizersJSONFile(t, filepath.Join(base, manifest.Fixtures[key]))
	}
	for _, want := range []string{
		"finance.normalizers.statements.canonicalize",
		"finance.normalizers.ratios.canonicalize",
		"finance.normalizers.market.canonicalize",
		"finance.normalizers.news.canonicalize",
		"finance.normalizers.sec.canonicalize",
		"finance.normalizers.peer.canonicalize",
		"finance.normalizers.provenance.attach",
		"finance.normalizers.symbol.normalize",
		"finance.normalizers.date.normalize",
		"finance.normalizers.currency.normalize",
		"finance.normalizers.corporate_actions.adjust",
		"finance.normalizers.unit_currency.envelope",
		"finance.normalizers.validation.error_taxonomy",
		"finance.normalizers.field_policy.stale",
		"finance.normalizers.field_policy.missing",
		"finance.normalizers.ordering.deterministic",
	} {
		if !contains(manifest.Capabilities, want) {
			t.Fatalf("capabilities missing %q: %#v", want, manifest.Capabilities)
		}
	}
	if manifest.FieldCanonicalization["case"] != "snake_case" ||
		manifest.FieldCanonicalization["currency"] != "ISO-4217 uppercase" ||
		manifest.FieldCanonicalization["typed_boundary"] == "" ||
		manifest.FieldCanonicalization["conversion_envelope"] == "" ||
		manifest.StaleMissingPolicy["missing_required_policy"] != "reject_record" ||
		manifest.StaleMissingPolicy["missing_optional_policy"] != "emit_null_with_missing_reason" ||
		manifest.StaleMissingPolicy["warning_marker"] != "finance_normalizer_stale_field" ||
		manifest.StaleMissingPolicy["missing_reason_required_when_null"] != true {
		t.Fatalf("canonicalization/stale/missing policy is too weak: fields=%#v stale=%#v", manifest.FieldCanonicalization, manifest.StaleMissingPolicy)
	}
	for _, domain := range wantDomains {
		if len(manifest.DeterministicOrdering[domain]) == 0 {
			t.Fatalf("deterministic ordering missing for %q", domain)
		}
	}
}

func TestFinRobotFinanceNormalizersTypedBoundaryContract(t *testing.T) {
	base := financeNormalizersLivePackageDir(t)

	var contract financeNormalizersContract
	decodeFinanceNormalizersJSONFile(t, filepath.Join(base, "contracts", "finance_normalizers_contract.json"), &contract)

	for _, want := range []string{"normalization_boundary", "corporate_actions", "validation"} {
		if !contains(contract.Domains, want) {
			t.Fatalf("contract domains missing %q: %#v", want, contract.Domains)
		}
	}
	for _, want := range []string{
		"symbol",
		"date",
		"currency",
		"missing_value",
		"unit_currency_envelope",
	} {
		if !contains(contract.TypedNormalizationBoundary.Covers, want) {
			t.Fatalf("typed boundary contract missing %q: %#v", want, contract.TypedNormalizationBoundary)
		}
	}
	if !contract.TypedNormalizationBoundary.ProviderFree ||
		contract.TypedNormalizationBoundary.Schema != "typed_normalization_boundary_v1" {
		t.Fatalf("typed boundary contract must stay provider-free: %#v", contract.TypedNormalizationBoundary)
	}
	for _, want := range []string{"adjustment_factor", "raw_close", "adjusted_close", "raw_volume", "adjusted_volume", "adjustment_method"} {
		if !contains(contract.CorporateActionAdjustment.RequiredAdjustmentFields, want) {
			t.Fatalf("corporate action contract missing %q: %#v", want, contract.CorporateActionAdjustment)
		}
	}
	if !contract.CorporateActionAdjustment.ProviderFree ||
		contract.CorporateActionAdjustment.Schema != "corporate_action_adjustment_v1" {
		t.Fatalf("corporate action contract must stay provider-free: %#v", contract.CorporateActionAdjustment)
	}
	for _, want := range []string{"currency_conversion_rate_missing", "date_not_rfc3339", "missing_required_field", "optional_value_missing", "symbol_not_canonical", "unit_scale_missing"} {
		if !contains(contract.ValidationErrorTaxonomy.ErrorCodes, want) {
			t.Fatalf("validation taxonomy missing %q: %#v", want, contract.ValidationErrorTaxonomy)
		}
	}
	if !contract.ValidationErrorTaxonomy.ProviderFree ||
		contract.ValidationErrorTaxonomy.Schema != "validation_error_taxonomy_v1" {
		t.Fatalf("validation taxonomy contract must stay provider-free: %#v", contract.ValidationErrorTaxonomy)
	}
}

func TestFinRobotFinanceNormalizersTypedBoundaryFixtures(t *testing.T) {
	base := financeNormalizersLivePackageDir(t)

	var boundary struct {
		ProviderFree bool             `json:"provider_free"`
		Rows         []map[string]any `json:"rows"`
	}
	decodeFinanceNormalizersJSONFile(t, filepath.Join(base, "fixtures", "typed_normalization_boundary_fixture.json"), &boundary)
	if !boundary.ProviderFree {
		t.Fatalf("typed boundary fixture must be provider-free")
	}
	boundaryTypes := map[string]bool{}
	for _, row := range boundary.Rows {
		boundaryTypes[financeNormalizerStringKey(row["boundary_type"])] = true
		if row["boundary_type"] == "unit_currency_envelope" {
			for _, field := range []string{"unit", "scale", "currency", "source_currency", "target_currency", "conversion_rate", "precision"} {
				if row[field] == nil {
					t.Fatalf("unit/currency envelope missing %q: %#v", field, row)
				}
			}
		}
		if row["boundary_type"] == "missing_value" && row["missing_reason"] == nil {
			t.Fatalf("missing value boundary must carry missing_reason: %#v", row)
		}
	}
	for _, want := range []string{"symbol", "date", "currency", "missing_value", "unit_currency_envelope"} {
		if !boundaryTypes[want] {
			t.Fatalf("typed boundary fixture missing %q: %#v", want, boundaryTypes)
		}
	}

	var corporateActions struct {
		ProviderFree bool             `json:"provider_free"`
		Rows         []map[string]any `json:"rows"`
	}
	decodeFinanceNormalizersJSONFile(t, filepath.Join(base, "fixtures", "corporate_action_adjustment_fixture.json"), &corporateActions)
	if !corporateActions.ProviderFree || len(corporateActions.Rows) < 2 {
		t.Fatalf("corporate action fixture invalid: %#v", corporateActions)
	}
	actionTypes := map[string]bool{}
	for _, row := range corporateActions.Rows {
		actionTypes[financeNormalizerStringKey(row["action_type"])] = true
		for _, field := range []string{"adjustment_factor", "raw_close", "adjusted_close", "raw_volume", "adjusted_volume", "adjustment_method"} {
			if row[field] == nil {
				t.Fatalf("corporate action row missing %q: %#v", field, row)
			}
		}
	}
	if !actionTypes["split"] || !actionTypes["cash_dividend"] {
		t.Fatalf("corporate action fixture must cover split and dividend adjustments: %#v", actionTypes)
	}

	var taxonomy struct {
		ProviderFree bool             `json:"provider_free"`
		Rows         []map[string]any `json:"rows"`
	}
	decodeFinanceNormalizersJSONFile(t, filepath.Join(base, "fixtures", "validation_error_taxonomy_fixture.json"), &taxonomy)
	if !taxonomy.ProviderFree {
		t.Fatalf("validation taxonomy fixture must be provider-free")
	}
	errorCodes := map[string]bool{}
	for _, row := range taxonomy.Rows {
		errorCodes[financeNormalizerStringKey(row["error_code"])] = true
		if row["policy"] != "reject_record" && row["policy"] != "emit_null_with_missing_reason" {
			t.Fatalf("validation taxonomy policy not stable: %#v", row)
		}
	}
	for _, want := range []string{"currency_conversion_rate_missing", "date_not_rfc3339", "missing_required_field", "optional_value_missing", "symbol_not_canonical", "unit_scale_missing"} {
		if !errorCodes[want] {
			t.Fatalf("validation taxonomy fixture missing %q: %#v", want, errorCodes)
		}
	}
}

func TestFinRobotFinanceNormalizersSchemasFixturesAndOrdering(t *testing.T) {
	base := financeNormalizersLivePackageDir(t)
	manifest := loadFinanceNormalizersManifest(t, base)

	for key, schemaPath := range manifest.Schemas {
		var schema financeNormalizerSchema
		decodeFinanceNormalizersJSONFile(t, filepath.Join(base, schemaPath), &schema)
		if schema.SchemaVersion != 1 || schema.ID == "" {
			t.Fatalf("%s schema header incomplete: %#v", key, schema)
		}
		if key != "policy" {
			for _, want := range []string{"schema", "source_id", "record_hash"} {
				if !contains(schema.Required, want) {
					t.Fatalf("%s schema required fields missing %q: %#v", key, want, schema.Required)
				}
			}
			for _, want := range []string{"as_of", "stale_after_days", "stale"} {
				if key != "provenance" && !contains(schema.Required, want) {
					t.Fatalf("%s schema stale fields missing %q: %#v", key, want, schema.Required)
				}
			}
			if schema.MissingRequiredPolicy != "" && schema.MissingRequiredPolicy != "reject_record" {
				t.Fatalf("%s missing required policy = %q", key, schema.MissingRequiredPolicy)
			}
			if schema.MissingOptionalPolicy != "" && schema.MissingOptionalPolicy != "emit_null_with_missing_reason" {
				t.Fatalf("%s missing optional policy = %q", key, schema.MissingOptionalPolicy)
			}
			assertFinanceNormalizerSnakeCase(t, append(append([]string{}, schema.Required...), schema.CanonicalFields...))
			if !reflect.DeepEqual(schema.DeterministicOrder, manifest.DeterministicOrdering[schema.Domain]) {
				t.Fatalf("%s deterministic order = %#v, want %#v", key, schema.DeterministicOrder, manifest.DeterministicOrdering[schema.Domain])
			}
		}
	}

	for key, fixturePath := range manifest.Fixtures {
		if key == "index" || key == "policy" {
			continue
		}
		var fixture struct {
			ProviderFree       bool             `json:"provider_free"`
			Rows               []map[string]any `json:"rows"`
			DeterministicOrder []string         `json:"deterministic_order"`
		}
		decodeFinanceNormalizersJSONFile(t, filepath.Join(base, fixturePath), &fixture)
		if !fixture.ProviderFree || len(fixture.Rows) == 0 {
			t.Fatalf("%s fixture header/rows invalid: %#v", key, fixture)
		}
		for i, row := range fixture.Rows {
			if row["schema"] == "" || row["source_id"] == "" || row["record_hash"] == "" {
				t.Fatalf("%s row %d missing schema/source/hash: %#v", key, i, row)
			}
			for field := range row {
				assertFinanceNormalizerSnakeCase(t, []string{field})
			}
			if row["stale"] == true && row["stale_after_days"] == nil {
				t.Fatalf("%s row %d stale without stale_after_days: %#v", key, i, row)
			}
			for field, value := range row {
				if value == nil && row["missing_reason"] == nil {
					t.Fatalf("%s row %d has null %q without missing_reason: %#v", key, i, field, row)
				}
			}
		}
		if !financeNormalizerRowsSorted(fixture.Rows, fixture.DeterministicOrder) {
			t.Fatalf("%s fixture rows are not sorted by %#v", key, fixture.DeterministicOrder)
		}
	}
}

func TestFinRobotFinanceNormalizersNoLiveProviders(t *testing.T) {
	base := financeNormalizersLivePackageDir(t)
	manifest := loadFinanceNormalizersManifest(t, base)

	var files []string
	for _, rel := range manifest.Entrypoints {
		files = append(files, filepath.Join(base, rel))
	}
	for _, rel := range manifest.Schemas {
		files = append(files, filepath.Join(base, rel))
	}
	for _, rel := range manifest.Fixtures {
		files = append(files, filepath.Join(base, rel))
	}
	files = append(files, filepath.Join(base, "contracts", "finance_normalizers_contract.json"))

	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(data)
		for _, blocked := range manifest.BlockedImports {
			if strings.Contains(text, "import "+blocked) ||
				strings.Contains(text, `require("`+blocked+`"`) ||
				strings.Contains(text, "from "+blocked+" import") {
				t.Fatalf("%s appears to import blocked provider dependency %q", path, blocked)
			}
		}
		for _, networkMarker := range []string{"https://", "http://"} {
			if strings.Contains(text, networkMarker) {
				t.Fatalf("%s contains live network locator %q", path, networkMarker)
			}
		}
	}
}

func financeNormalizersLivePackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "finance_normalizers")
}

func loadFinanceNormalizersManifest(t *testing.T, base string) financeNormalizersManifest {
	t.Helper()
	var manifest financeNormalizersManifest
	decodeFinanceNormalizersJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	return manifest
}

func assertFinanceNormalizersJSONOrLeiaFile(t *testing.T, path string) {
	t.Helper()
	if strings.HasSuffix(path, ".json") {
		assertFinanceNormalizersJSONFile(t, path)
		return
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
}

func assertFinanceNormalizersJSONFile(t *testing.T, path string) {
	t.Helper()
	var v any
	decodeFinanceNormalizersJSONFile(t, path, &v)
}

func decodeFinanceNormalizersJSONFile(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func assertFinanceNormalizerSnakeCase(t *testing.T, fields []string) {
	t.Helper()
	re := regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	for _, field := range fields {
		if !re.MatchString(field) {
			t.Fatalf("field %q is not snake_case", field)
		}
	}
}

func financeNormalizerRowsSorted(rows []map[string]any, order []string) bool {
	if len(order) == 0 {
		return false
	}
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		var parts []string
		for _, field := range order {
			parts = append(parts, financeNormalizerStringKey(row[field]))
		}
		keys = append(keys, strings.Join(parts, "\x00"))
	}
	return sort.StringsAreSorted(keys)
}

func financeNormalizerStringKey(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case float64:
		return strings.TrimRight(strings.TrimRight(jsonNumberString(value), "0"), ".")
	default:
		return ""
	}
}

func jsonNumberString(value float64) string {
	data, _ := json.Marshal(value)
	return string(data)
}
