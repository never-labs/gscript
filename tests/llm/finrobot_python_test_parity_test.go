package leia_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type finrobotPythonParityManifest struct {
	SchemaVersion         int                         `json:"schema_version"`
	ManifestID            string                      `json:"manifest_id"`
	FixtureVersion        string                      `json:"fixture_version"`
	ProviderFree          bool                        `json:"provider_free"`
	LiveNetwork           bool                        `json:"live_network"`
	RealDependencyImports bool                        `json:"real_dependency_imports"`
	PythonRuntimeRequired bool                        `json:"python_runtime_required"`
	SourceScope           string                      `json:"source_scope"`
	CoverageEntries       []finrobotPythonParityEntry `json:"coverage_entries"`
}

type finrobotPythonParityEntry struct {
	ID                     string         `json:"id"`
	SourcePythonTests      []string       `json:"source_python_tests"`
	FixturePath            string         `json:"fixture_path"`
	FixtureSHA256          string         `json:"fixture_sha256"`
	AssertionGroups        []string       `json:"assertion_groups"`
	ExpectedFields         []string       `json:"expected_fields"`
	ExpectedArrayMinCounts map[string]int `json:"expected_array_min_counts"`
}

func TestFinRobotPythonTestParityCoverageManifest(t *testing.T) {
	base := finrobotPythonParityFixtureDir(t)
	manifest := loadFinRobotPythonParityManifest(t, base)

	if manifest.SchemaVersion != 1 || manifest.ManifestID != "finrobot-python-test-parity-fixtures" {
		t.Fatalf("manifest header = schema %d id %q", manifest.SchemaVersion, manifest.ManifestID)
	}
	if manifest.FixtureVersion != "finrobot-python-test-parity-v1" {
		t.Fatalf("fixture version = %q", manifest.FixtureVersion)
	}
	if !manifest.ProviderFree || manifest.LiveNetwork || manifest.RealDependencyImports || manifest.PythonRuntimeRequired {
		t.Fatalf("manifest must stay provider-free and runtime-free: %#v", manifest)
	}
	if manifest.SourceScope != "finrobot_equity/core/tests/*" {
		t.Fatalf("source scope = %q", manifest.SourceScope)
	}

	gotIDs := make([]string, 0, len(manifest.CoverageEntries))
	for _, entry := range manifest.CoverageEntries {
		gotIDs = append(gotIDs, entry.ID)
		if len(entry.SourcePythonTests) == 0 || entry.FixturePath == "" || entry.FixtureSHA256 == "" || len(entry.AssertionGroups) == 0 || len(entry.ExpectedFields) == 0 {
			t.Fatalf("coverage entry incomplete: %#v", entry)
		}
		for _, source := range entry.SourcePythonTests {
			if !strings.HasPrefix(source, "finrobot_equity/core/tests/") || !strings.HasSuffix(source, ".py") {
				t.Fatalf("%s source test path outside Python parity scope: %q", entry.ID, source)
			}
		}
	}
	sort.Strings(gotIDs)
	wantIDs := []string{"chart_report_artifact", "financial_data_processor", "report_structure", "valuation_engine", "web_product_state"}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("coverage ids = %#v, want %#v", gotIDs, wantIDs)
	}
}

func TestFinRobotPythonTestParityFixtureChecksumsAndExpectedFields(t *testing.T) {
	base := finrobotPythonParityFixtureDir(t)
	manifest := loadFinRobotPythonParityManifest(t, base)

	for _, entry := range manifest.CoverageEntries {
		entry := entry
		t.Run(entry.ID, func(t *testing.T) {
			path := filepath.Join(base, entry.FixturePath)
			assertFinRobotPythonParitySHA256(t, path, entry.FixtureSHA256)
			var fixture map[string]any
			decodeFinRobotPythonParityJSONFile(t, path, &fixture)

			if fixture["provider_free"] != true || fixture["live_network"] != false || fixture["real_dependency_imports"] != false {
				t.Fatalf("%s must be provider-free/offline/no runtime imports: %#v", entry.ID, fixture)
			}
			for _, field := range entry.ExpectedFields {
				if _, ok := fixture[field]; !ok {
					t.Fatalf("%s missing expected field %q", entry.ID, field)
				}
			}
			for field, minCount := range entry.ExpectedArrayMinCounts {
				items, ok := fixture[field].([]any)
				if !ok {
					t.Fatalf("%s expected array field %q, got %#v", entry.ID, field, fixture[field])
				}
				if len(items) < minCount {
					t.Fatalf("%s.%s length = %d, want at least %d", entry.ID, field, len(items), minCount)
				}
			}
			provenance, ok := fixture["provenance"].(map[string]any)
			if !ok || provenance["provider"] != "fixture" || provenance["replay_ready"] != true || provenance["source_url_redacted"] != true {
				t.Fatalf("%s provenance must be replay-ready fixture provenance: %#v", entry.ID, fixture["provenance"])
			}
		})
	}
}

func TestFinRobotPythonTestParityRepresentativeAssertions(t *testing.T) {
	base := finrobotPythonParityFixtureDir(t)

	var financial struct {
		NormalizedRows []struct {
			Metric string  `json:"metric"`
			Value  float64 `json:"value"`
		} `json:"normalized_rows"`
		AssertionIntent struct {
			RequiredMetricsPresent []string `json:"required_metrics_present"`
		} `json:"assertion_intent"`
	}
	decodeFinRobotPythonParityJSONFile(t, filepath.Join(base, "fixtures", "financial_data_processor_ACME_fixture.json"), &financial)
	metrics := map[string]float64{}
	for _, row := range financial.NormalizedRows {
		metrics[row.Metric] = row.Value
	}
	for _, metric := range financial.AssertionIntent.RequiredMetricsPresent {
		if _, ok := metrics[metric]; !ok {
			t.Fatalf("financial processor fixture missing required metric %q", metric)
		}
	}
	if metrics["market_price"] != 42.75 || metrics["free_cash_flow"] != 104.6 {
		t.Fatalf("financial processor representative values = %#v", metrics)
	}

	var valuation struct {
		MethodOutputs []struct {
			Method   string             `json:"method"`
			Expected map[string]float64 `json:"expected"`
		} `json:"method_outputs"`
		TargetPriceSynthesis struct {
			TargetPrice float64 `json:"target_price"`
			RatingBand  string  `json:"rating_band"`
		} `json:"target_price_synthesis"`
	}
	decodeFinRobotPythonParityJSONFile(t, filepath.Join(base, "fixtures", "valuation_engine_ACME_fixture.json"), &valuation)
	prices := map[string]float64{}
	for _, output := range valuation.MethodOutputs {
		prices[output.Method] = output.Expected["price_per_share"]
	}
	assertFinRobotPythonParityFloat(t, "dcf price", prices["dcf"], 43.61604, 0.000001)
	assertFinRobotPythonParityFloat(t, "ev/ebitda price", prices["ev_ebitda"], 50.2, 0.000001)
	assertFinRobotPythonParityFloat(t, "p/e price", prices["pe"], 53.2, 0.000001)
	assertFinRobotPythonParityFloat(t, "target price", valuation.TargetPriceSynthesis.TargetPrice, 47.50802, 0.000001)
	if valuation.TargetPriceSynthesis.RatingBand != "neutral" {
		t.Fatalf("rating band = %q", valuation.TargetPriceSynthesis.RatingBand)
	}

	var report struct {
		OrderedSections  []string `json:"ordered_sections"`
		SectionContracts []struct {
			ID             string   `json:"id"`
			RequiredFields []string `json:"required_fields"`
			SourceRefs     []string `json:"source_refs"`
		} `json:"section_contracts"`
	}
	decodeFinRobotPythonParityJSONFile(t, filepath.Join(base, "fixtures", "report_structure_ACME_fixture.json"), &report)
	wantSections := []string{"company_snapshot", "financial_data", "valuation", "risk_factors", "investment_thesis", "appendix"}
	if !reflect.DeepEqual(report.OrderedSections, wantSections) {
		t.Fatalf("report section order = %#v, want %#v", report.OrderedSections, wantSections)
	}
	for _, section := range report.SectionContracts {
		if len(section.RequiredFields) == 0 || len(section.SourceRefs) == 0 {
			t.Fatalf("section contract missing required fields/source refs: %#v", section)
		}
	}

	var artifact struct {
		CleanSkipWithoutRenderer bool `json:"clean_skip_without_renderer"`
		ChartArtifact            struct {
			Format       string   `json:"format"`
			Width        int      `json:"width"`
			Height       int      `json:"height"`
			SnapshotHash string   `json:"snapshot_hash"`
			SourceRefs   []string `json:"source_refs"`
		} `json:"chart_artifact"`
		ReportArtifacts []struct {
			Format string `json:"format"`
			URI    string `json:"uri"`
		} `json:"report_artifacts"`
	}
	decodeFinRobotPythonParityJSONFile(t, filepath.Join(base, "fixtures", "chart_report_artifact_ACME_fixture.json"), &artifact)
	if !artifact.CleanSkipWithoutRenderer || artifact.ChartArtifact.Format != "svg" || artifact.ChartArtifact.Width <= 0 || artifact.ChartArtifact.Height <= 0 || artifact.ChartArtifact.SnapshotHash == "" || len(artifact.ChartArtifact.SourceRefs) == 0 {
		t.Fatalf("chart artifact assertion fields incomplete: %#v", artifact)
	}
	formats := map[string]bool{}
	for _, item := range artifact.ReportArtifacts {
		formats[item.Format] = true
		if !strings.HasPrefix(item.URI, "fixture://") {
			t.Fatalf("artifact URI must be fixture scoped: %#v", item)
		}
	}
	if !formats["text/html"] || !formats["application/pdf"] {
		t.Fatalf("report artifacts missing HTML/PDF: %#v", artifact.ReportArtifacts)
	}

	var product struct {
		Routes []struct {
			ID           string `json:"id"`
			RequiresAuth bool   `json:"requires_auth"`
		} `json:"routes"`
		Sessions []struct {
			ID        string `json:"id"`
			State     string `json:"state"`
			CSRFValid bool   `json:"csrf_valid"`
		} `json:"sessions"`
		ReportRequestLifecycle []struct {
			Sequence int    `json:"sequence"`
			To       string `json:"to"`
		} `json:"report_request_lifecycle"`
		DownloadAuthorization []struct {
			SessionID string `json:"session_id"`
			Decision  string `json:"decision"`
			Reason    string `json:"reason"`
		} `json:"download_authorization"`
	}
	decodeFinRobotPythonParityJSONFile(t, filepath.Join(base, "fixtures", "web_product_state_ACME_fixture.json"), &product)
	if len(product.Routes) < 5 || !product.Routes[1].RequiresAuth {
		t.Fatalf("web routes missing auth contract: %#v", product.Routes)
	}
	for i, transition := range product.ReportRequestLifecycle {
		if transition.Sequence != i+1 {
			t.Fatalf("lifecycle sequence at %d = %d", i, transition.Sequence)
		}
	}
	if product.ReportRequestLifecycle[len(product.ReportRequestLifecycle)-1].To != "completed" {
		t.Fatalf("lifecycle final state = %#v", product.ReportRequestLifecycle)
	}
	deniedExpired := false
	for _, decision := range product.DownloadAuthorization {
		if decision.SessionID == "sess_expired" && decision.Decision == "deny" && decision.Reason == "expired_session" {
			deniedExpired = true
		}
	}
	if !deniedExpired {
		t.Fatalf("download authorization should deny expired sessions: %#v", product.DownloadAuthorization)
	}
}

func finrobotPythonParityFixtureDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "regression_fixtures", "finrobot_python_test_parity")
}

func loadFinRobotPythonParityManifest(t *testing.T, base string) finrobotPythonParityManifest {
	t.Helper()
	var manifest finrobotPythonParityManifest
	decodeFinRobotPythonParityJSONFile(t, filepath.Join(base, "coverage_manifest.json"), &manifest)
	return manifest
}

func decodeFinRobotPythonParityJSONFile(t *testing.T, path string, out any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
}

func assertFinRobotPythonParitySHA256(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != want {
		t.Fatalf("%s sha256 = %s, want %s", path, got, want)
	}
}

func assertFinRobotPythonParityFloat(t *testing.T, label string, got, want, tolerance float64) {
	t.Helper()
	if math.Abs(got-want) > tolerance {
		t.Fatalf("%s = %s, want %s within %s", label, fmt.Sprint(got), fmt.Sprint(want), fmt.Sprint(tolerance))
	}
}
