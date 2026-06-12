package leia_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestGenericEvidenceVerificationLivePackageContractFixtureClosedLoop(t *testing.T) {
	base := genericEvidenceVerificationPackageDir(t)

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
		NoBuiltInGuarantee struct {
			Required  bool   `json:"required"`
			Statement string `json:"statement"`
		} `json:"no_built_in_guarantee"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, "package.manifest.json"), &manifest)
	if manifest.SchemaVersion != 1 ||
		manifest.ID != "generic-evidence-verification" ||
		manifest.PackageName != "leia-generic-ai-evidence-verification" ||
		manifest.PackageBoundaryID != "generic-ai-evidence-verification" ||
		manifest.CapabilityID != "generic.ai.evidence.verify" {
		t.Fatalf("unexpected manifest identity: %#v", manifest)
	}
	if !manifest.ProviderFree || manifest.DomainSpecific || manifest.LiveNetworkDefault || manifest.LiveModelDefault || manifest.DependsOnQRuntime {
		t.Fatalf("manifest must stay provider-free/generic/offline: %#v", manifest)
	}
	statement := strings.ToLower(manifest.NoBuiltInGuarantee.Statement)
	if !manifest.NoBuiltInGuarantee.Required ||
		!strings.Contains(statement, "leia core") ||
		!strings.Contains(statement, "does not provide") ||
		!strings.Contains(statement, "built-in") ||
		!strings.Contains(statement, manifest.PackageName) ||
		!strings.Contains(statement, "package boundary") {
		t.Fatalf("manifest missing no-built-in boundary: %#v", manifest.NoBuiltInGuarantee)
	}
	for _, want := range []string{
		"generic.ai.evidence.verify",
		"generic.ai.evidence.claim_record",
		"generic.ai.evidence.source_ref",
		"generic.ai.evidence.citation_ref",
		"generic.ai.evidence.requirement_matrix",
		"generic.ai.evidence.citation_normalize",
		"generic.ai.evidence.freshness_check",
		"generic.ai.evidence.unresolved_ref",
		"generic.ai.evidence.quality_summary",
		"generic.ai.evidence.clean_degradation",
	} {
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
		LiveModel             bool              `json:"live_model"`
		LiveModelCalls        bool              `json:"live_model_calls"`
		RealDependencyImports bool              `json:"real_dependency_imports"`
		RequiresCredentials   bool              `json:"requires_credentials"`
		ProviderSDKsRequired  bool              `json:"provider_sdks_required"`
		DependsOnQRuntime     bool              `json:"depends_on_q_runtime"`
		FieldContracts        map[string]string `json:"field_contracts"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, manifest.Contracts["contract"]), &contract)
	if contract.SchemaVersion != 1 || contract.PackageBoundaryID != manifest.PackageBoundaryID ||
		contract.PackageName != "generic.ai.evidence.verify" || contract.Entrypoint != "ai.evidence.verify" ||
		!contract.ProviderFree || contract.DomainSpecific || contract.LiveNetwork || contract.LiveModel ||
		contract.LiveModelCalls || contract.RealDependencyImports || contract.RequiresCredentials ||
		contract.ProviderSDKsRequired || contract.DependsOnQRuntime {
		t.Fatalf("contract boundary mismatch: %#v", contract)
	}
	for _, want := range []string{"claim_records", "source_annotations", "citation_refs", "requirement_matrix", "citation_normalization", "freshness_policy", "clean_degradation"} {
		if contract.FieldContracts[want] == "" {
			t.Fatalf("contract field_contracts missing %q: %#v", want, contract.FieldContracts)
		}
	}

	var index struct {
		ProviderFree          bool `json:"provider_free"`
		LiveNetwork           bool `json:"live_network"`
		RealDependencyImports bool `json:"real_dependency_imports"`
		Fixtures              []struct {
			FixtureKey            string         `json:"fixture_key"`
			Capability            string         `json:"capability"`
			Path                  string         `json:"path"`
			Schema                string         `json:"schema"`
			ProviderFree          bool           `json:"provider_free"`
			LiveNetwork           bool           `json:"live_network"`
			RealDependencyImports bool           `json:"real_dependency_imports"`
			Metadata              map[string]any `json:"metadata"`
		} `json:"fixtures"`
	}
	decodeDocumentPipelineJSONFile(t, filepath.Join(base, manifest.Fixtures["index"]), &index)
	if !index.ProviderFree || index.LiveNetwork || index.RealDependencyImports || len(index.Fixtures) != 2 {
		t.Fatalf("fixture index header/count mismatch: %#v", index)
	}
	for _, fixture := range index.Fixtures {
		if !fixture.ProviderFree || fixture.LiveNetwork || fixture.RealDependencyImports || fixture.Path == "" || fixture.Schema == "" {
			t.Fatalf("fixture index entry is not provider-free/offline: %#v", fixture)
		}
	}
}

func TestGenericEvidenceVerificationLivePackageFixtureShape(t *testing.T) {
	base := genericEvidenceVerificationPackageDir(t)
	fixture := loadGenericEvidenceVerificationFixture(t, filepath.Join(base, "fixtures", "evidence_verification_fixture.json"))
	if !fixture.ProviderFree || fixture.DomainSpecific || fixture.LiveNetwork || fixture.RealDependencyImports || fixture.LiveModelCalls {
		t.Fatalf("fixture must stay provider-free and offline: %#v", fixture)
	}
	if len(fixture.ClaimRecords) != 3 || len(fixture.SourceAnnotations) != 4 ||
		len(fixture.NormalizedCitations) != 4 || len(fixture.VerificationResults) != 3 ||
		len(fixture.FreshnessWarnings) != 1 || len(fixture.UnresolvedRefs) != 1 ||
		len(fixture.CleanDegradationActions) != 1 {
		t.Fatalf("fixture counts drifted: claims=%d sources=%d citations=%d results=%d freshness=%d unresolved=%d degradation=%d",
			len(fixture.ClaimRecords), len(fixture.SourceAnnotations), len(fixture.NormalizedCitations),
			len(fixture.VerificationResults), len(fixture.FreshnessWarnings), len(fixture.UnresolvedRefs),
			len(fixture.CleanDegradationActions))
	}
	sourceIDs := map[string]bool{}
	for _, source := range fixture.SourceAnnotations {
		if source.SourceID == "" || source.Kind == "" || source.Locator == "" || source.EvidenceHash == "" || !source.ProviderFree {
			t.Fatalf("source annotation incomplete: %#v", source)
		}
		sourceIDs[source.SourceID] = true
	}
	for _, citation := range fixture.NormalizedCitations {
		if citation.ClaimID == "" || citation.SourceID == "" || citation.NormalizedSourceID == "" || citation.FirstSeenOrder == 0 {
			t.Fatalf("normalized citation incomplete: %#v", citation)
		}
		if citation.Resolved && !sourceIDs[citation.SourceID] {
			t.Fatalf("normalized citation source_id %q does not resolve", citation.SourceID)
		}
	}
	for _, result := range fixture.VerificationResults {
		if result.ClaimID == "" || result.Status == "" || result.Action == "" {
			t.Fatalf("verification result incomplete: %#v", result)
		}
		for _, ref := range result.ResolvedSourceRefs {
			if !sourceIDs[ref] {
				t.Fatalf("verification result resolved ref %q does not resolve", ref)
			}
		}
	}
	action := fixture.CleanDegradationActions[0]
	if action.Mode != "omit_claim_and_warn" || action.InventedClaimAllowed || !action.ProviderFree || action.LiveNetwork {
		t.Fatalf("clean degradation action drifted: %#v", action)
	}
	summary := fixture.EvidenceQualitySummary
	if summary.ClaimsTotal != 3 || summary.ClaimsVerified != 2 || summary.ClaimsDegraded != 1 ||
		summary.UnresolvedRefsTotal != 1 || summary.FreshnessWarningsTotal != 1 || !summary.ProviderFree {
		t.Fatalf("quality summary drifted: %#v", summary)
	}
}

func TestGenericEvidenceVerificationLivePackageIsDomainNeutral(t *testing.T) {
	base := genericEvidenceVerificationPackageDir(t)
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lower := strings.ToLower(string(data))
		for _, forbidden := range []string{"finrobot", "acme", "aapl", "ticker", "equity", "investment", "valuation", "dcf", "target_price", "sec.gov", "10-k", "10-q", "finance.", "financial", "portfolio", "market"} {
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

func TestGenericEvidenceVerificationLivePackageSchemaRequiredFields(t *testing.T) {
	base := genericEvidenceVerificationPackageDir(t)
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "evidence_policy_v1.schema.json"), []string{"policy_id", "provider_free", "live_network", "minimum_source_refs_per_claim", "reject_unresolved_refs", "requires_source_quote_or_metric", "allowed_evidence_kinds", "freshness_policy", "citation_normalization"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "evidence_requirement_matrix_v1.schema.json"), []string{"matrix_id", "provider_free", "live_network", "rows"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "citation_normalization_v1.schema.json"), []string{"normalization_id", "provider_free", "live_network", "source_id_case", "strip_query_fragments_from_locator", "dedupe_repeated_refs", "preserve_first_seen_order", "normalized_citations"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "evidence_verification_result_v1.schema.json"), []string{"schema_version", "fixture_key", "provider_free", "live_network", "real_dependency_imports", "live_model_calls", "policy", "requirement_matrix", "claim_records", "source_annotations", "normalized_citations", "freshness_warnings", "resolved_refs", "unresolved_refs", "unsupported_claims", "verification_results", "evidence_quality_summary", "clean_degradation_actions"})
	assertDocumentPipelineSchemaRequired(t, filepath.Join(base, "schemas", "evidence_degradation_action_v1.schema.json"), []string{"action_id", "claim_id", "mode", "reason", "provider_free", "live_network", "invented_claim_allowed", "warning_refs"})
}

func TestGenericEvidenceVerificationLivePackageExecutableSkeleton(t *testing.T) {
	path := filepath.Join(genericEvidenceVerificationPackageDir(t), "main.leia")
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
			got, err := vm.Get("generic_evidence_verification_live_package_summary")
			if err != nil {
				t.Fatalf("Get generic_evidence_verification_live_package_summary: %v", err)
			}
			want := "generic_evidence_verification_live_package capability=generic.ai.evidence.verify entrypoint=ai.evidence.verify claims=3 sources=4 citations=4 results=3 freshness_warnings=1 unresolved=1 clean_degradation=1 provider_free=true live_network=false imports=false model_calls=false"
			if got != want {
				t.Fatalf("summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

func genericEvidenceVerificationPackageDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "live_packages", "generic_evidence_verification")
}

type genericEvidenceVerificationFixture struct {
	ProviderFree          bool `json:"provider_free"`
	DomainSpecific        bool `json:"domain_specific"`
	LiveNetwork           bool `json:"live_network"`
	RealDependencyImports bool `json:"real_dependency_imports"`
	LiveModelCalls        bool `json:"live_model_calls"`
	SourceAnnotations     []struct {
		SourceID     string `json:"source_id"`
		Kind         string `json:"kind"`
		Locator      string `json:"locator"`
		EvidenceHash string `json:"evidence_hash"`
		ProviderFree bool   `json:"provider_free"`
	} `json:"source_annotations"`
	ClaimRecords []struct {
		ClaimID      string   `json:"claim_id"`
		ClaimKind    string   `json:"claim_kind"`
		SourceRefs   []string `json:"source_refs"`
		CitationRefs []string `json:"citation_refs"`
	} `json:"claim_records"`
	NormalizedCitations []struct {
		ClaimID            string `json:"claim_id"`
		SourceID           string `json:"source_id"`
		NormalizedSourceID string `json:"normalized_source_id"`
		FirstSeenOrder     int    `json:"first_seen_order"`
		Resolved           bool   `json:"resolved"`
	} `json:"normalized_citations"`
	FreshnessWarnings []struct {
		WarningID string `json:"warning_id"`
		SourceID  string `json:"source_id"`
	} `json:"freshness_warnings"`
	UnresolvedRefs []struct {
		ClaimID  string `json:"claim_id"`
		SourceID string `json:"source_id"`
	} `json:"unresolved_refs"`
	VerificationResults []struct {
		ClaimID              string   `json:"claim_id"`
		Status               string   `json:"status"`
		ResolvedSourceRefs   []string `json:"resolved_source_refs"`
		UnresolvedRefs       []string `json:"unresolved_refs"`
		FreshnessWarningRefs []string `json:"freshness_warning_refs"`
		Action               string   `json:"action"`
	} `json:"verification_results"`
	EvidenceQualitySummary struct {
		ClaimsTotal            int  `json:"claims_total"`
		ClaimsVerified         int  `json:"claims_verified"`
		ClaimsDegraded         int  `json:"claims_degraded"`
		UnresolvedRefsTotal    int  `json:"unresolved_refs_total"`
		FreshnessWarningsTotal int  `json:"freshness_warnings_total"`
		ProviderFree           bool `json:"provider_free"`
	} `json:"evidence_quality_summary"`
	CleanDegradationActions []struct {
		ActionID             string   `json:"action_id"`
		ClaimID              string   `json:"claim_id"`
		Mode                 string   `json:"mode"`
		ProviderFree         bool     `json:"provider_free"`
		LiveNetwork          bool     `json:"live_network"`
		InventedClaimAllowed bool     `json:"invented_claim_allowed"`
		WarningRefs          []string `json:"warning_refs"`
	} `json:"clean_degradation_actions"`
}

func loadGenericEvidenceVerificationFixture(t *testing.T, path string) genericEvidenceVerificationFixture {
	t.Helper()
	var fixture genericEvidenceVerificationFixture
	decodeDocumentPipelineJSONFile(t, path, &fixture)
	return fixture
}
