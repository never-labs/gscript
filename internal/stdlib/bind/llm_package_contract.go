package bind

import "fmt"

func llmProviderFreePackageContractValue(src, opts *Table) *Table {
	out := NewTable()
	if src != nil {
		for _, key := range src.PairsKeysSnapshot() {
			out.RawSet(key, llmCloneValue(src.RawGet(key)))
		}
	}
	out.RawSetString("__llm_package_contract", BoolValue(true))
	out.RawSetString("kind", StringValue("provider_free_package_contract"))
	out.RawSetString("schema_version", IntValue(int64(llmReplayOptionInt(src, "schema_version", llmReplayOptionInt(opts, "schema_version", 1)))))
	out.RawSetString("version", StringValue(llmReplayOptionString(src, "version", llmReplayOptionString(opts, "version", "provider_free_package_contract.v1"))))
	packageID := llmReplayOptionString(src, "package_id", llmReplayOptionString(src, "id", llmReplayOptionString(opts, "package_id", "package")))
	out.RawSetString("package_id", StringValue(packageID))
	if out.RawGetString("id").IsNil() {
		out.RawSetString("id", StringValue(packageID))
	}
	if out.RawGetString("package_name").IsNil() {
		out.RawSetString("package_name", StringValue(llmReplayOptionString(opts, "package_name", packageID)))
	}
	out.RawSetString("provider_free", BoolValue(llmReplayOptionBool(src, "provider_free", llmReplayOptionBool(opts, "provider_free", true))))
	out.RawSetString("offline_verifiable", BoolValue(llmReplayOptionBool(src, "offline_verifiable", llmReplayOptionBool(opts, "offline_verifiable", true))))
	out.RawSetString("domain_specific", BoolValue(llmReplayOptionBool(src, "domain_specific", llmReplayOptionBool(opts, "domain_specific", false))))
	liveNetwork := llmPackageContractBool(src, opts, "live_network", "live_network_default", false)
	realImports := llmPackageContractBool(src, opts, "real_dependency_imports", "real_dependency_import_default", false)
	liveModel := llmPackageContractBool(src, opts, "live_model", "live_model_default", false)
	credentialsRequired := llmPackageContractBool(src, opts, "credentials_required", "provider_credentials_required", false)
	out.RawSetString("live_network", BoolValue(liveNetwork))
	out.RawSetString("live_network_default", BoolValue(liveNetwork))
	out.RawSetString("real_dependency_imports", BoolValue(realImports))
	out.RawSetString("real_dependency_import_default", BoolValue(realImports))
	out.RawSetString("live_model", BoolValue(liveModel))
	out.RawSetString("live_model_default", BoolValue(liveModel))
	out.RawSetString("credentials_required", BoolValue(credentialsRequired))
	out.RawSetString("provider_credentials_required", BoolValue(credentialsRequired))
	out.RawSetString("secret_values_present", BoolValue(llmReplayOptionBool(src, "secret_values_present", llmReplayOptionBool(opts, "secret_values_present", false))))
	out.RawSetString("default_policy", TableValue(llmPackageContractDefaultPolicy(src, opts)))
	out.RawSetString("credentials", TableValue(llmPackageContractCredentials(src, opts)))
	out.RawSetString("entrypoint_count", IntValue(int64(llmPackageContractTableCount(out.RawGetString("entrypoints").Table()))))
	out.RawSetString("schema_count", IntValue(int64(llmPackageContractTableCount(out.RawGetString("schemas").Table()))))
	out.RawSetString("fixture_count", IntValue(int64(llmPackageContractTableCount(out.RawGetString("fixtures").Table()))))
	out.RawSetString("capability_count", IntValue(int64(llmPackageContractTableCount(out.RawGetString("capabilities").Table()))))
	out.RawSetString("summary", TableValue(llmPackageContractSummary(out)))
	return out
}

func llmPackageContractDefaultPolicy(src, opts *Table) *Table {
	policy := NewTable()
	if src != nil {
		if existing := src.RawGetString("default_policy"); existing.IsTable() {
			llmCopyTable(policy, existing.Table(), true)
		}
	}
	if opts != nil {
		if existing := opts.RawGetString("default_policy"); existing.IsTable() {
			llmCopyTable(policy, existing.Table(), false)
		}
	}
	policy.RawSetString("mode", StringValue(llmReplayOptionString(policy, "mode", llmReplayModeFixture)))
	policy.RawSetString("live_network", BoolValue(llmReplayOptionBool(policy, "live_network", false)))
	policy.RawSetString("provider_credentials_required", BoolValue(llmReplayOptionBool(policy, "provider_credentials_required", false)))
	policy.RawSetString("real_dependency_imports", BoolValue(llmReplayOptionBool(policy, "real_dependency_imports", false)))
	policy.RawSetString("clean_skip_without_dependency", BoolValue(llmReplayOptionBool(policy, "clean_skip_without_dependency", true)))
	return policy
}

func llmPackageContractCredentials(src, opts *Table) *Table {
	credentials := NewTable()
	if src != nil {
		if existing := src.RawGetString("credentials"); existing.IsTable() {
			llmCopyTable(credentials, existing.Table(), true)
		}
	}
	if opts != nil {
		if existing := opts.RawGetString("credentials"); existing.IsTable() {
			llmCopyTable(credentials, existing.Table(), false)
		}
	}
	for _, field := range []string{"required", "optional", "required_env", "optional_env", "secret_env_patterns", "secret_refs"} {
		if credentials.RawGetString(field).IsNil() {
			credentials.RawSetString(field, TableValue(NewSequentialArrayTable(0)))
		}
	}
	credentials.RawSetString("provider_credentials_required", BoolValue(llmReplayOptionBool(credentials, "provider_credentials_required", false)))
	return credentials
}

func llmPackageContractSummary(contract *Table) *Table {
	summary := NewTable()
	for _, field := range []string{
		"package_id",
		"package_name",
		"provider_free",
		"offline_verifiable",
		"domain_specific",
		"live_network",
		"live_network_default",
		"real_dependency_imports",
		"real_dependency_import_default",
		"live_model",
		"live_model_default",
		"credentials_required",
		"entrypoint_count",
		"schema_count",
		"fixture_count",
		"capability_count",
	} {
		if value := contract.RawGetString(field); !value.IsNil() {
			summary.RawSetString(field, llmCloneValue(value))
		}
	}
	return summary
}

func llmValidatePackageContractValue(contract, opts *Table) *Table {
	findings := NewSequentialArrayTable(0)
	if llmReplayOptionBool(opts, "require_provider_free", true) && !llmReplayOptionBool(contract, "provider_free", false) {
		llmPackageContractFinding(findings, "provider_free", "package contract must be provider-free", "")
	}
	if llmReplayOptionBool(opts, "require_offline", true) {
		if llmReplayOptionBool(contract, "live_network", llmReplayOptionBool(contract, "live_network_default", false)) {
			llmPackageContractFinding(findings, "live_network_default", "package contract must default live network off", "")
		}
		if llmReplayOptionBool(contract, "real_dependency_imports", llmReplayOptionBool(contract, "real_dependency_import_default", false)) {
			llmPackageContractFinding(findings, "real_dependency_import_default", "package contract must default real dependency imports off", "")
		}
		if llmReplayOptionBool(contract, "live_model", llmReplayOptionBool(contract, "live_model_default", false)) {
			llmPackageContractFinding(findings, "live_model_default", "package contract must default live model calls off", "")
		}
		if llmReplayOptionBool(contract, "credentials_required", llmReplayOptionBool(contract, "provider_credentials_required", false)) {
			llmPackageContractFinding(findings, "provider_credentials_required", "package contract must not require provider credentials by default", "")
		}
	}
	llmValidatePackageDefaultPolicy(contract.RawGetString("default_policy").Table(), findings)
	llmValidatePackageCredentials(contract.RawGetString("credentials").Table(), findings)
	for _, field := range []string{"entrypoints", "schemas", "fixtures"} {
		llmValidatePackageReferenceTable(contract.RawGetString(field).Table(), field, findings)
	}
	if llmReplayOptionBool(opts, "require_entrypoints", false) && llmPackageContractTableCount(contract.RawGetString("entrypoints").Table()) == 0 {
		llmPackageContractFinding(findings, "entrypoints", "package contract must include entrypoints", "")
	}
	if llmReplayOptionBool(opts, "require_fixture_index", false) && !llmPackageContractHasFixtureIndex(contract) {
		llmPackageContractFinding(findings, "fixture_index", "package contract must reference a fixture index", "")
	}
	ok := findings.Length() == 0
	out := NewTable()
	out.RawSetString("kind", StringValue("package_contract_validation"))
	out.RawSetString("schema_version", IntValue(1))
	out.RawSetString("version", StringValue("package_contract_validation.v1"))
	out.RawSetString("ok", BoolValue(ok))
	if ok {
		out.RawSetString("status", StringValue("ok"))
		out.RawSetString("result_status", StringValue("ok"))
	} else {
		out.RawSetString("status", StringValue("failed"))
		out.RawSetString("result_status", StringValue("blocked"))
	}
	out.RawSetString("package_id", llmCloneValue(contract.RawGetString("package_id")))
	out.RawSetString("finding_count", IntValue(int64(findings.Length())))
	out.RawSetString("findings", TableValue(findings))
	out.RawSetString("summary", TableValue(llmPackageContractSummary(contract)))
	return out
}

func llmValidatePackageDefaultPolicy(policy *Table, findings *Table) {
	if policy == nil {
		llmPackageContractFinding(findings, "default_policy", "package contract must include default policy", "")
		return
	}
	if mode := llmReplayOptionString(policy, "mode", ""); mode != llmReplayModeFixture {
		llmPackageContractFinding(findings, "default_policy.mode", "default policy mode must be fixture_replay", "")
	}
	for _, check := range []struct {
		field string
		want  bool
	}{
		{"live_network", false},
		{"provider_credentials_required", false},
		{"real_dependency_imports", false},
	} {
		value := policy.RawGetString(check.field)
		if !value.IsNil() && (!value.IsBool() || value.Bool() != check.want) {
			llmPackageContractFinding(findings, "default_policy."+check.field, fmt.Sprintf("default policy %s must be %v", check.field, check.want), "")
		}
	}
}

func llmPackageContractBool(src, opts *Table, primary, secondary string, fallback bool) bool {
	if src != nil {
		if value := src.RawGetString(primary); !value.IsNil() {
			return value.Truthy()
		}
		if value := src.RawGetString(secondary); !value.IsNil() {
			return value.Truthy()
		}
	}
	if opts != nil {
		if value := opts.RawGetString(primary); !value.IsNil() {
			return value.Truthy()
		}
		if value := opts.RawGetString(secondary); !value.IsNil() {
			return value.Truthy()
		}
	}
	return fallback
}

func llmValidatePackageCredentials(credentials *Table, findings *Table) {
	if credentials == nil {
		return
	}
	if llmReplayOptionBool(credentials, "provider_credentials_required", false) {
		llmPackageContractFinding(findings, "credentials.provider_credentials_required", "credentials must not require provider credentials by default", "")
	}
	for _, field := range []string{"required", "required_env", "secret_refs"} {
		value := credentials.RawGetString(field)
		if value.IsTable() && llmPackageContractTableCount(value.Table()) > 0 {
			llmPackageContractFinding(findings, "credentials."+field, "provider-free package contract must not require credential refs", "")
		}
	}
}

func llmValidatePackageReferenceTable(refs *Table, field string, findings *Table) {
	if refs == nil {
		return
	}
	for _, key := range refs.PairsKeysSnapshot() {
		value := refs.RawGet(key)
		label := field
		if key.IsString() {
			label = field + "." + key.Str()
		}
		llmValidatePackageReferenceValue(value, label, findings)
	}
}

func llmValidatePackageReferenceValue(value Value, field string, findings *Table) {
	if value.IsString() {
		llmValidatePackageReferenceString(value.Str(), field, findings)
		return
	}
	if value.IsTable() {
		for _, item := range llmPackageContractValues(value.Table()) {
			llmValidatePackageReferenceValue(item, field, findings)
		}
		return
	}
	if !value.IsNil() {
		llmPackageContractFinding(findings, field, "package reference must be a string or string table", "")
	}
}

func llmValidatePackageReferenceString(ref, field string, findings *Table) {
	if ref == "" {
		llmPackageContractFinding(findings, field, "package reference must be non-empty", "")
		return
	}
	llmValidateFixtureIndexReferenceString(findings, field, ref, "")
}

func llmPackageContractHasFixtureIndex(contract *Table) bool {
	for _, field := range []string{"entrypoints", "fixtures"} {
		table := contract.RawGetString(field).Table()
		if table == nil {
			continue
		}
		for _, key := range []string{"fixture_index", "index"} {
			if value := table.RawGetString(key); value.IsString() && value.Str() != "" {
				return true
			}
		}
	}
	return false
}

func llmPackageContractTableCount(table *Table) int {
	if table == nil {
		return 0
	}
	if table.Length() > 0 {
		return table.Length()
	}
	return len(table.PairsKeysSnapshot())
}

func llmPackageContractValues(table *Table) []Value {
	if table == nil {
		return nil
	}
	values := make([]Value, 0, llmPackageContractTableCount(table))
	for i := 1; i <= table.Length(); i++ {
		values = append(values, table.RawGet(IntValue(int64(i))))
	}
	if len(values) > 0 {
		return values
	}
	for _, key := range table.PairsKeysSnapshot() {
		values = append(values, table.RawGet(key))
	}
	return values
}

func llmPackageContractFinding(findings *Table, kind, message, field string) {
	finding := NewTable()
	finding.RawSetString("kind", StringValue(kind))
	finding.RawSetString("message", StringValue(message))
	if field != "" {
		finding.RawSetString("field", StringValue(field))
	}
	findings.RawSet(IntValue(int64(findings.Length()+1)), TableValue(finding))
}
