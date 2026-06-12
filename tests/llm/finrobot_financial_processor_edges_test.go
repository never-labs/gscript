package leia_test

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type financialProcessorEdgeFixture struct {
	SchemaVersion                    int                          `json:"schema_version"`
	ID                               string                       `json:"id"`
	FixtureKey                       string                       `json:"fixture_key"`
	ProviderFree                     bool                         `json:"provider_free"`
	LiveNetwork                      bool                         `json:"live_network"`
	RealDependencyImports            bool                         `json:"real_dependency_imports"`
	FinancialNumberCleaningCases     []financialNumberCleaning    `json:"financial_number_cleaning_cases"`
	PDFTableConfig                   pdfTableConfig               `json:"pdf_table_config"`
	ForecastGrowthCases              []forecastGrowthCase         `json:"forecast_growth_cases"`
	CurrencyPeriodNormalizationCases []currencyPeriodCase         `json:"currency_period_normalization_cases"`
	NormalizedStatementRows          []map[string]any             `json:"normalized_statement_rows"`
	DeterministicOrder               []string                     `json:"deterministic_order"`
	AssertionIntent                  financialEdgeAssertionIntent `json:"assertion_intent"`
}

type financialNumberCleaning struct {
	ID              string   `json:"id"`
	RawValue        string   `json:"raw_value"`
	ColumnUnit      string   `json:"column_unit"`
	NormalizedValue *float64 `json:"normalized_value"`
	Unit            string   `json:"unit"`
	Scale           float64  `json:"scale"`
	Currency        *string  `json:"currency"`
	IsMissing       bool     `json:"is_missing"`
	MissingReason   string   `json:"missing_reason"`
}

type pdfTableConfig struct {
	ProviderFree        bool             `json:"provider_free"`
	SourceRef           string           `json:"source_ref"`
	Engine              string           `json:"engine"`
	Pages               []string         `json:"pages"`
	TableAreas          []string         `json:"table_areas"`
	Columns             []string         `json:"columns"`
	HeaderRows          int              `json:"header_rows"`
	DropEmptyRows       bool             `json:"drop_empty_rows"`
	NormalizeWhitespace bool             `json:"normalize_whitespace"`
	ExpectedRows        []map[string]any `json:"expected_rows"`
}

type forecastGrowthCase struct {
	Metric        string   `json:"metric"`
	FromPeriod    string   `json:"from_period"`
	ToPeriod      string   `json:"to_period"`
	PriorValue    *float64 `json:"prior_value"`
	CurrentValue  *float64 `json:"current_value"`
	Growth        *float64 `json:"growth"`
	MissingReason string   `json:"missing_reason"`
}

type currencyPeriodCase struct {
	RawCurrency  string `json:"raw_currency"`
	Currency     string `json:"currency"`
	RawPeriod    string `json:"raw_period"`
	FiscalYear   int    `json:"fiscal_year"`
	FiscalPeriod string `json:"fiscal_period"`
	Period       string `json:"period"`
}

type financialEdgeAssertionIntent struct {
	Covers                    []string `json:"covers"`
	RequiredNormalizedMetrics []string `json:"required_normalized_metrics"`
	ProviderFree              bool     `json:"provider_free"`
}

func TestFinRobotFinancialProcessorEdgeFixtures(t *testing.T) {
	path := filepath.Join(financeNormalizersLivePackageDir(t), "fixtures", "financial_data_processor_edge_cases_fixture.json")
	var fixture financialProcessorEdgeFixture
	decodeFinanceNormalizersJSONFile(t, path, &fixture)

	if fixture.SchemaVersion != 1 || fixture.ID != "financial_data_processor_edge_cases_fixture" {
		t.Fatalf("fixture header = schema %d id %q", fixture.SchemaVersion, fixture.ID)
	}
	if !fixture.ProviderFree || fixture.LiveNetwork || fixture.RealDependencyImports || !fixture.AssertionIntent.ProviderFree {
		t.Fatalf("fixture must stay provider-free/offline: %#v", fixture)
	}
	assertFinancialProcessorFixtureNoLiveRuntime(t, path)
	assertFinancialProcessorEdgeCoverage(t, fixture.AssertionIntent.Covers)
	assertFinancialNumberCleaningCases(t, fixture.FinancialNumberCleaningCases)
	assertFinancialProcessorPDFTableConfig(t, fixture.PDFTableConfig)
	assertFinancialProcessorForecastGrowth(t, fixture.ForecastGrowthCases)
	assertFinancialProcessorCurrencyPeriods(t, fixture.CurrencyPeriodNormalizationCases)
	assertFinancialProcessorStatementRows(t, fixture)
}

func assertFinancialProcessorEdgeCoverage(t *testing.T, covers []string) {
	t.Helper()
	want := []string{
		"financial_number_cleaning",
		"pdf_table_config",
		"forecast_growth",
		"missing_values",
		"negative_numbers",
		"unit_conversion",
		"currency_normalization",
		"period_normalization",
		"deterministic_ordering",
	}
	for _, key := range want {
		if !contains(covers, key) {
			t.Fatalf("edge fixture coverage missing %q: %#v", key, covers)
		}
	}
}

func assertFinancialNumberCleaningCases(t *testing.T, cases []financialNumberCleaning) {
	t.Helper()
	if len(cases) < 7 {
		t.Fatalf("number cleaning cases = %d, want at least 7", len(cases))
	}
	byID := map[string]financialNumberCleaning{}
	for _, c := range cases {
		byID[c.ID] = c
		if c.IsMissing {
			if c.NormalizedValue != nil || c.MissingReason == "" {
				t.Fatalf("missing numeric case must carry null value and reason: %#v", c)
			}
			continue
		}
		if c.NormalizedValue == nil {
			t.Fatalf("non-missing numeric case has nil value: %#v", c)
		}
		if c.Unit == "currency" && (c.Currency == nil || *c.Currency != strings.ToUpper(*c.Currency)) {
			t.Fatalf("currency case must normalize ISO currency uppercase: %#v", c)
		}
	}
	assertFinancialFloat(t, "currency_commas", *byID["currency_commas"].NormalizedValue, 1234.5, 0)
	assertFinancialFloat(t, "parentheses_negative", *byID["parentheses_negative"].NormalizedValue, -56.7, 0)
	assertFinancialFloat(t, "unicode_minus_percent", *byID["unicode_minus_percent"].NormalizedValue, -0.125, 0)
	assertFinancialFloat(t, "billions_suffix", *byID["billions_suffix"].NormalizedValue, 1200000000, 0)
	assertFinancialFloat(t, "millions_column_unit", *byID["millions_column_unit"].NormalizedValue, 3400000000, 0)
	if !byID["dash_missing"].IsMissing || !byID["na_missing"].IsMissing {
		t.Fatalf("dash/N/A cases must be explicit missing values: %#v", byID)
	}
}

func assertFinancialProcessorPDFTableConfig(t *testing.T, config pdfTableConfig) {
	t.Helper()
	if !config.ProviderFree || config.Engine != "fixture_pdf_table_parser" {
		t.Fatalf("pdf table config must use fixture parser: %#v", config)
	}
	if len(config.Pages) != 2 || len(config.TableAreas) != 1 || config.HeaderRows != 1 ||
		!config.DropEmptyRows || !config.NormalizeWhitespace {
		t.Fatalf("pdf table config missing pandas-equivalent table settings: %#v", config)
	}
	wantColumns := []string{"line_item", "FY2024", "FY2025", "FY2026E"}
	if !reflect.DeepEqual(config.Columns, wantColumns) {
		t.Fatalf("pdf columns = %#v, want %#v", config.Columns, wantColumns)
	}
	if len(config.ExpectedRows) != 2 || config.ExpectedRows[1]["fy2026e"] != nil ||
		config.ExpectedRows[1]["missing_reason"] != "forecast_not_disclosed" {
		t.Fatalf("pdf expected rows must preserve missing forecast reason: %#v", config.ExpectedRows)
	}
	if got := config.ExpectedRows[1]["fy2025"]; got != float64(-205700000) {
		t.Fatalf("pdf expected negative value = %#v, want -205700000", got)
	}
}

func assertFinancialProcessorForecastGrowth(t *testing.T, cases []forecastGrowthCase) {
	t.Helper()
	if len(cases) != 3 {
		t.Fatalf("forecast growth cases = %d, want 3", len(cases))
	}
	for _, c := range cases {
		if c.PriorValue == nil || c.CurrentValue == nil || c.Growth == nil {
			if c.Growth != nil || c.MissingReason == "" {
				t.Fatalf("missing forecast growth must carry null growth and reason: %#v", c)
			}
			continue
		}
		want := (*c.CurrentValue / *c.PriorValue) - 1
		assertFinancialFloat(t, c.Metric+" "+c.ToPeriod, *c.Growth, want, 0.0000005)
	}
}

func assertFinancialProcessorCurrencyPeriods(t *testing.T, cases []currencyPeriodCase) {
	t.Helper()
	if len(cases) != 3 {
		t.Fatalf("currency/period cases = %d, want 3", len(cases))
	}
	for _, c := range cases {
		if c.Currency != strings.ToUpper(c.Currency) || len(c.Currency) != 3 {
			t.Fatalf("currency was not normalized to ISO uppercase: %#v", c)
		}
		if c.FiscalYear < 2000 || c.FiscalPeriod == "" || strings.Contains(c.Period, " ") {
			t.Fatalf("period was not normalized: %#v", c)
		}
	}
	if cases[0].Period != "FY2025" || cases[1].Period != "2026Q1" || cases[2].Period != "2026Q1" {
		t.Fatalf("period normalization cases = %#v", cases)
	}
}

func assertFinancialProcessorStatementRows(t *testing.T, fixture financialProcessorEdgeFixture) {
	t.Helper()
	if !reflect.DeepEqual(fixture.DeterministicOrder, []string{"symbol", "fiscal_year", "fiscal_period", "statement_type", "line_item"}) {
		t.Fatalf("deterministic order = %#v", fixture.DeterministicOrder)
	}
	if !financeNormalizerRowsSorted(fixture.NormalizedStatementRows, fixture.DeterministicOrder) {
		t.Fatalf("normalized statement rows are not sorted by %#v", fixture.DeterministicOrder)
	}

	seenMetrics := map[string]bool{}
	negativeSeen := false
	missingSeen := false
	for i, row := range fixture.NormalizedStatementRows {
		if row["schema"] != "normalized_statement_v1" || row["source_id"] == "" || row["record_hash"] == "" {
			t.Fatalf("row %d missing normalized statement identity: %#v", i, row)
		}
		if row["currency"] != strings.ToUpper(row["currency"].(string)) {
			t.Fatalf("row %d currency not normalized: %#v", i, row)
		}
		seenMetrics[row["line_item"].(string)] = true
		if value, ok := row["value"].(float64); ok && value < 0 {
			negativeSeen = true
		}
		if row["value"] == nil {
			missingSeen = true
			if row["missing_reason"] == "" {
				t.Fatalf("row %d null value without missing_reason: %#v", i, row)
			}
		}
	}
	for _, metric := range fixture.AssertionIntent.RequiredNormalizedMetrics {
		if !seenMetrics[metric] {
			t.Fatalf("missing normalized metric %q in rows %#v", metric, fixture.NormalizedStatementRows)
		}
	}
	if !negativeSeen || !missingSeen {
		t.Fatalf("statement rows must include negative and explicit missing values: negative=%v missing=%v", negativeSeen, missingSeen)
	}
}

func assertFinancialProcessorFixtureNoLiveRuntime(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	text := string(data)
	for _, disallowed := range []string{"http://", "https://", "q/runtime", "import ", "require("} {
		if strings.Contains(text, disallowed) {
			t.Fatalf("%s contains disallowed live/runtime marker %q", path, disallowed)
		}
	}
}

func assertFinancialFloat(t *testing.T, label string, got, want, tolerance float64) {
	t.Helper()
	if tolerance == 0 {
		tolerance = 0.000000001
	}
	if math.Abs(got-want) > tolerance {
		t.Fatalf("%s = %.12f, want %.12f", label, got, want)
	}
}
