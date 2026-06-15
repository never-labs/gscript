package bind

import "testing"

func TestLLMProviderFreePackageContractNormalizesAndValidates(t *testing.T) {
	interp := runLLMTestProgram(t, `
contract := llm.provider_free_package_contract({
    id: "generic-ai-contract"
    package_name: "leia-generic-ai-contract"
    entrypoints: {
        smoke: "main.leia"
        fixture_index: "fixtures/provider_free_fixture_index.json"
    }
    schemas: {
        contract: "schemas/contract.schema.json"
    }
    fixtures: {
        index: "fixtures/provider_free_fixture_index.json"
    }
    capabilities: ["generic.ai.contract"]
}, {
    default_policy: {fixture_hook: "recorded_fixture"}
})
gate := llm.validate_package_contract(contract, {
    require_entrypoints: true
    require_fixture_index: true
})

alias_contract := llm.packageContract({
    package_id: "alias-contract"
    entrypoints: {fixture_index: "fixtures/index.json"}
})
alias_gate := llm.validatePackageContract(alias_contract, {require_fixture_index: true})

bad := llm.provider_free_package_contract({
    id: "bad-contract"
    provider_free: false
    live_network_default: true
    real_dependency_import_default: true
    provider_credentials_required: true
    default_policy: {
        mode: "fixture_replay"
        live_network: true
        provider_credentials_required: true
        real_dependency_imports: true
    }
    credentials: {
        provider_credentials_required: true
        required_env: ["OPENAI_API_KEY"]
    }
    entrypoints: {
        fixture_index: "../secrets.json"
    }
})
bad_gate := llm.validate_package_contract(bad, {require_fixture_index: true})

contract_marker := contract.__llm_package_contract
contract_kind := contract.kind
contract_version := contract.version
contract_id := contract.package_id
contract_name := contract.package_name
contract_provider_free := contract.provider_free
contract_live_network := contract.live_network_default
contract_imports := contract.real_dependency_import_default
contract_policy_mode := contract.default_policy.mode
contract_policy_live_network := contract.default_policy.live_network
contract_policy_hook := contract.default_policy.fixture_hook
contract_credentials_required_count := #contract.credentials.required
contract_entrypoint_count := contract.entrypoint_count
contract_schema_count := contract.schema_count
contract_fixture_count := contract.fixture_count
contract_capability_count := contract.capability_count
contract_summary_package := contract.summary.package_id
gate_ok := gate.ok
gate_status := gate.status
gate_findings := gate.finding_count
alias_gate_ok := alias_gate.ok
bad_ok := bad_gate.ok
bad_status := bad_gate.status
bad_findings := bad_gate.finding_count
bad_first_kind := bad_gate.findings[1].kind

missing_ok, missing_err := pcall(llm.provider_free_package_contract)
bad_opts_ok, bad_opts_err := pcall(llm.provider_free_package_contract, {}, "opts")
bad_validate_ok, bad_validate_err := pcall(llm.validate_package_contract, {}, "opts")
`, nil)

	for name, want := range map[string]Value{
		"contract_marker":                     BoolValue(true),
		"contract_kind":                       StringValue("provider_free_package_contract"),
		"contract_version":                    StringValue("provider_free_package_contract.v1"),
		"contract_id":                         StringValue("generic-ai-contract"),
		"contract_name":                       StringValue("leia-generic-ai-contract"),
		"contract_provider_free":              BoolValue(true),
		"contract_live_network":               BoolValue(false),
		"contract_imports":                    BoolValue(false),
		"contract_policy_mode":                StringValue("fixture_replay"),
		"contract_policy_live_network":        BoolValue(false),
		"contract_policy_hook":                StringValue("recorded_fixture"),
		"contract_credentials_required_count": IntValue(0),
		"contract_entrypoint_count":           IntValue(2),
		"contract_schema_count":               IntValue(1),
		"contract_fixture_count":              IntValue(1),
		"contract_capability_count":           IntValue(1),
		"contract_summary_package":            StringValue("generic-ai-contract"),
		"gate_ok":                             BoolValue(true),
		"gate_status":                         StringValue("ok"),
		"gate_findings":                       IntValue(0),
		"alias_gate_ok":                       BoolValue(true),
		"bad_ok":                              BoolValue(false),
		"bad_status":                          StringValue("failed"),
		"bad_first_kind":                      StringValue("provider_free"),
		"missing_ok":                          BoolValue(false),
		"bad_opts_ok":                         BoolValue(false),
		"bad_validate_ok":                     BoolValue(false),
	} {
		got := interp.GetGlobal(name)
		if !got.Equal(want) {
			t.Fatalf("%s = %v, want %v", name, got, want)
		}
	}
	if got := interp.GetGlobal("bad_findings"); !got.IsInt() || got.Int() < 6 {
		t.Fatalf("bad_findings = %v, want at least 6", got)
	}
	for _, name := range []string{"missing_err", "bad_opts_err", "bad_validate_err"} {
		if got := interp.GetGlobal(name); !got.IsString() || got.Str() == "" {
			t.Fatalf("%s = %v, want error string", name, got)
		}
	}
}
