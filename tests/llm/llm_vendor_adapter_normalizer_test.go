package leia_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestFinRobotVendorAdapterSkeleton(t *testing.T) {
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

			if err := vm.ExecFile(filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "vendor_adapters.leia")); err != nil {
				t.Fatalf("ExecFile: %v", err)
			}
			got, err := vm.Get("vendor_adapters_summary")
			if err != nil {
				t.Fatalf("Get vendor_adapters_summary: %v", err)
			}
			want := "vendor_adapters providers=6 fixtures=6 offline=true built_in_api=false fmp_schema=fundamental_metric_row_v1 reddit_terms=offline-fixture-only"
			if got != want {
				t.Fatalf("vendor_adapters_summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

func TestFinRobotFinanceNormalizerSkeleton(t *testing.T) {
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

			if err := vm.ExecFile(filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "finance_normalizers.leia")); err != nil {
				t.Fatalf("ExecFile: %v", err)
			}
			got, err := vm.Get("finance_normalizers_summary")
			if err != nil {
				t.Fatalf("Get finance_normalizers_summary: %v", err)
			}
			want := "finance_normalizers schemas=7 rows=7 valid=true stale_market_days=5 invalid_field=roic"
			if got != want {
				t.Fatalf("finance_normalizers_summary = %#v, want %#v", got, want)
			}
			if len(prints) != 1 || prints[0] != want {
				t.Fatalf("prints = %#v, want %q", prints, want)
			}
		})
	}
}

func TestFinRobotVendorAdapterMetadataIsFixtureOnly(t *testing.T) {
	path := filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "vendor_adapters.leia")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	source := string(data)
	for _, snippet := range []string{
		"FR-GAP-013",
		"offline_only: true",
		"live_network: false",
		"built_in_api: false",
		"YahooChartPayload",
		"FinnhubProfilePayload",
		"FMPKeyMetricsPayload",
		"SECFilingPayload",
		"EarningsTranscriptPayload",
		"RedditSearchPayload",
		"rate_limit:",
		"terms:",
		"env:FINNHUB_TOKEN",
		"env:FMP_API_KEY",
		"env:REDDIT_CLIENT_SECRET",
		"recorded_payload_fixtures",
	} {
		if !strings.Contains(source, snippet) {
			t.Fatalf("vendor_adapters.leia missing metadata evidence %q", snippet)
		}
	}
}

func TestFinRobotFinanceNormalizerMetadataPreservesProvenance(t *testing.T) {
	path := filepath.Join(repoRoot(t), "examples", "ai", "finrobot_translation", "finance_normalizers.leia")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	source := string(data)
	for _, snippet := range []string{
		"FR-GAP-014",
		"market_price_bar_v1",
		"fundamental_metric_row_v1",
		"sec_section_v1",
		"earnings_segment_v1",
		"social_sentiment_record_v1",
		"analyst_recommendation_v1",
		"provider",
		"replay_key",
		"captured_at",
		"source_schema",
		"missing_field",
		"stale_after_days",
	} {
		if !strings.Contains(source, snippet) {
			t.Fatalf("finance_normalizers.leia missing normalizer evidence %q", snippet)
		}
	}
}
