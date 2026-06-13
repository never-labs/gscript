package leia_test

import (
	"fmt"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

type finRobotFMPRowField struct {
	name string
	init string
	expr string
}

func execFinRobotFMPStableRowScript(t *testing.T, vm *leia.VM, prefix, endpoint string, fields []finRobotFMPRowField) error {
	t.Helper()
	var initializers strings.Builder
	var extraction strings.Builder
	for _, field := range fields {
		fmt.Fprintf(&initializers, "%s_%s := %s\n", prefix, field.name, field.init)
		fmt.Fprintf(&extraction, "            %s_%s = %s\n", prefix, field.name, field.expr)
	}
	return execFinRobotLiveDataScript(t, vm, fmt.Sprintf(`
%s_request_error := nil
%s_json_error := nil
%s_status := 0
%s_ok := false
%s

url := "https://financialmodelingprep.com/stable/%s?symbol=AAPL&apikey=" .. os.getenv("LEIA_FMP_API_KEY")
resp, err := net.get(url, {timeout: 30})
if err != nil {
    %s_request_error = err
} else {
    %s_status = resp.status
    %s_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            %s_json_error = json_err
        } else {
            row := data[1]
%s
        }
    } else {
        %s_request_error = resp.statusText
    }
}
`, prefix, prefix, prefix, prefix, initializers.String(), endpoint, prefix, prefix, prefix, prefix, extraction.String(), prefix))
}

func TestFinRobotLiveFMPCompanyProfileDataIntegration(t *testing.T) {
	vm := newFinRobotFMPLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
fmp_profile_request_error := nil
fmp_profile_json_error := nil
fmp_profile_status := 0
fmp_profile_ok := false
fmp_profile_symbol := ""
fmp_profile_name := ""
fmp_profile_exchange := ""
fmp_profile_sector := ""
fmp_profile_currency := ""

url := "https://financialmodelingprep.com/stable/profile?symbol=AAPL&apikey=" .. os.getenv("LEIA_FMP_API_KEY")
resp, err := net.get(url, {timeout: 30})
if err != nil {
    fmp_profile_request_error = err
} else {
    fmp_profile_status = resp.status
    fmp_profile_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            fmp_profile_json_error = json_err
        } else {
            row := data[1]
            fmp_profile_symbol = row.symbol
            fmp_profile_name = row.companyName
            fmp_profile_exchange = row.exchange
            fmp_profile_sector = row.sector
            fmp_profile_currency = row.currency
        }
    } else {
        fmp_profile_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotLiveDataOK(t, vm, "FMP profile", "fmp_profile_status", "fmp_profile_request_error", "fmp_profile_json_error", "fmp_profile_ok")
	symbol := mustGetString(t, vm, "fmp_profile_symbol")
	name := mustGetString(t, vm, "fmp_profile_name")
	exchange := mustGetString(t, vm, "fmp_profile_exchange")
	sector := mustGetString(t, vm, "fmp_profile_sector")
	currency := mustGetString(t, vm, "fmp_profile_currency")
	fmt.Printf("fmp_profile symbol=%q name=%q exchange=%q sector=%q currency=%q\n", symbol, name, exchange, sector, currency)
	if symbol != "AAPL" || !strings.Contains(strings.ToLower(name), "apple") || exchange == "" || sector == "" || currency == "" {
		t.Fatalf("unexpected FMP profile payload: symbol=%q name=%q exchange=%q sector=%q currency=%q", symbol, name, exchange, sector, currency)
	}
}

func TestFinRobotLiveFMPHistoricalPriceLightDataIntegration(t *testing.T) {
	vm := newFinRobotFMPLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
fmp_price_request_error := nil
fmp_price_json_error := nil
fmp_price_status := 0
fmp_price_ok := false
fmp_price_date := ""
fmp_price_close := 0.0
fmp_price_volume := 0

url := "https://financialmodelingprep.com/stable/historical-price-eod/light?symbol=AAPL&apikey=" .. os.getenv("LEIA_FMP_API_KEY")
resp, err := net.get(url, {timeout: 30})
if err != nil {
    fmp_price_request_error = err
} else {
    fmp_price_status = resp.status
    fmp_price_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            fmp_price_json_error = json_err
        } else {
            row := data[1]
            fmp_price_date = row.date
            fmp_price_close = row.price
            fmp_price_volume = row.volume
        }
    } else {
        fmp_price_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotLiveDataOK(t, vm, "FMP historical-price-eod light", "fmp_price_status", "fmp_price_request_error", "fmp_price_json_error", "fmp_price_ok")
	date := mustGetString(t, vm, "fmp_price_date")
	closePrice := mustGetFloat(t, vm, "fmp_price_close")
	volume := mustGetInt(t, vm, "fmp_price_volume")
	fmt.Printf("fmp_price date=%q price=%f volume=%d\n", date, closePrice, volume)
	if date == "" || closePrice <= 0 || volume < 0 {
		t.Fatalf("unexpected FMP historical-price-eod light payload: date=%q price=%f volume=%d", date, closePrice, volume)
	}
}

func TestFinRobotLiveFMPEnterpriseValuesDataIntegration(t *testing.T) {
	vm := newFinRobotFMPLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
fmp_ev_request_error := nil
fmp_ev_json_error := nil
fmp_ev_status := 0
fmp_ev_ok := false
fmp_ev_symbol := ""
fmp_ev_date := ""
fmp_ev_market_cap := 0.0
fmp_ev_enterprise_value := 0.0

url := "https://financialmodelingprep.com/stable/enterprise-values?symbol=AAPL&apikey=" .. os.getenv("LEIA_FMP_API_KEY")
resp, err := net.get(url, {timeout: 30})
if err != nil {
    fmp_ev_request_error = err
} else {
    fmp_ev_status = resp.status
    fmp_ev_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            fmp_ev_json_error = json_err
        } else {
            row := data[1]
            fmp_ev_symbol = row.symbol
            fmp_ev_date = row.date
            fmp_ev_market_cap = row.marketCapitalization
            fmp_ev_enterprise_value = row.enterpriseValue
        }
    } else {
        fmp_ev_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotLiveDataOK(t, vm, "FMP enterprise-values", "fmp_ev_status", "fmp_ev_request_error", "fmp_ev_json_error", "fmp_ev_ok")
	symbol := mustGetString(t, vm, "fmp_ev_symbol")
	date := mustGetString(t, vm, "fmp_ev_date")
	marketCap := mustGetFloat(t, vm, "fmp_ev_market_cap")
	enterpriseValue := mustGetFloat(t, vm, "fmp_ev_enterprise_value")
	fmt.Printf("fmp_enterprise_values symbol=%q date=%q market_cap=%f enterprise_value=%f\n", symbol, date, marketCap, enterpriseValue)
	if symbol != "AAPL" || date == "" || marketCap <= 0 || enterpriseValue <= 0 {
		t.Fatalf("unexpected FMP enterprise-values payload: symbol=%q date=%q market_cap=%f enterprise_value=%f", symbol, date, marketCap, enterpriseValue)
	}
}

func TestFinRobotLiveFMPKeyMetricsDataIntegration(t *testing.T) {
	vm := newFinRobotFMPLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
fmp_metrics_request_error := nil
fmp_metrics_json_error := nil
fmp_metrics_status := 0
fmp_metrics_ok := false
fmp_metrics_symbol := ""
fmp_metrics_date := ""
fmp_metrics_revenue_per_share := 0.0
fmp_metrics_pe_ratio := 0.0
fmp_metrics_market_cap := 0.0

url := "https://financialmodelingprep.com/stable/key-metrics?symbol=AAPL&apikey=" .. os.getenv("LEIA_FMP_API_KEY")
resp, err := net.get(url, {timeout: 30})
if err != nil {
    fmp_metrics_request_error = err
} else {
    fmp_metrics_status = resp.status
    fmp_metrics_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            fmp_metrics_json_error = json_err
        } else {
            row := data[1]
            fmp_metrics_symbol = row.symbol
            fmp_metrics_date = row.date
            fmp_metrics_revenue_per_share = row.revenuePerShare
            fmp_metrics_pe_ratio = row.peRatio
            fmp_metrics_market_cap = row.marketCap
        }
    } else {
        fmp_metrics_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotLiveDataOK(t, vm, "FMP key-metrics", "fmp_metrics_status", "fmp_metrics_request_error", "fmp_metrics_json_error", "fmp_metrics_ok")
	symbol := mustGetString(t, vm, "fmp_metrics_symbol")
	date := mustGetString(t, vm, "fmp_metrics_date")
	revenuePerShare := mustGetFloat(t, vm, "fmp_metrics_revenue_per_share")
	peRatio := mustGetFloat(t, vm, "fmp_metrics_pe_ratio")
	marketCap := mustGetFloat(t, vm, "fmp_metrics_market_cap")
	fmt.Printf("fmp_key_metrics symbol=%q date=%q revenue_per_share=%f pe_ratio=%f market_cap=%f\n", symbol, date, revenuePerShare, peRatio, marketCap)
	if symbol != "AAPL" || date == "" || revenuePerShare <= 0 || peRatio <= 0 || marketCap <= 0 {
		t.Fatalf("unexpected FMP key-metrics payload: symbol=%q date=%q revenue_per_share=%f pe_ratio=%f market_cap=%f", symbol, date, revenuePerShare, peRatio, marketCap)
	}
}

func TestFinRobotLiveFMPRatiosDataIntegration(t *testing.T) {
	vm := newFinRobotFMPLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
fmp_ratios_request_error := nil
fmp_ratios_json_error := nil
fmp_ratios_status := 0
fmp_ratios_ok := false
fmp_ratios_symbol := ""
fmp_ratios_date := ""
fmp_ratios_gross_margin := 0.0
fmp_ratios_current_ratio := 0.0
fmp_ratios_debt_equity := 0.0

url := "https://financialmodelingprep.com/stable/ratios?symbol=AAPL&apikey=" .. os.getenv("LEIA_FMP_API_KEY")
resp, err := net.get(url, {timeout: 30})
if err != nil {
    fmp_ratios_request_error = err
} else {
    fmp_ratios_status = resp.status
    fmp_ratios_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            fmp_ratios_json_error = json_err
        } else {
            row := data[1]
            fmp_ratios_symbol = row.symbol
            fmp_ratios_date = row.date
            fmp_ratios_gross_margin = row.grossProfitMargin
            fmp_ratios_current_ratio = row.currentRatio
            fmp_ratios_debt_equity = row.debtEquityRatio
        }
    } else {
        fmp_ratios_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotLiveDataOK(t, vm, "FMP ratios", "fmp_ratios_status", "fmp_ratios_request_error", "fmp_ratios_json_error", "fmp_ratios_ok")
	symbol := mustGetString(t, vm, "fmp_ratios_symbol")
	date := mustGetString(t, vm, "fmp_ratios_date")
	grossMargin := mustGetFloat(t, vm, "fmp_ratios_gross_margin")
	currentRatio := mustGetFloat(t, vm, "fmp_ratios_current_ratio")
	debtEquity := mustGetFloat(t, vm, "fmp_ratios_debt_equity")
	fmt.Printf("fmp_ratios symbol=%q date=%q gross_margin=%f current_ratio=%f debt_equity=%f\n", symbol, date, grossMargin, currentRatio, debtEquity)
	if symbol != "AAPL" || date == "" || grossMargin <= 0 || grossMargin > 1 || currentRatio <= 0 || debtEquity < 0 {
		t.Fatalf("unexpected FMP ratios payload: symbol=%q date=%q gross_margin=%f current_ratio=%f debt_equity=%f", symbol, date, grossMargin, currentRatio, debtEquity)
	}
}

func TestFinRobotLiveFMPRatingsSnapshotDataIntegration(t *testing.T) {
	vm := newFinRobotFMPLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
fmp_ratings_request_error := nil
fmp_ratings_json_error := nil
fmp_ratings_status := 0
fmp_ratings_ok := false
fmp_ratings_symbol := ""
fmp_ratings_rating := ""
fmp_ratings_overall_score := 0.0
fmp_ratings_dcf_score := 0.0

url := "https://financialmodelingprep.com/stable/ratings-snapshot?symbol=AAPL&apikey=" .. os.getenv("LEIA_FMP_API_KEY")
resp, err := net.get(url, {timeout: 30})
if err != nil {
    fmp_ratings_request_error = err
} else {
    fmp_ratings_status = resp.status
    fmp_ratings_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            fmp_ratings_json_error = json_err
        } else {
            row := data[1]
            fmp_ratings_symbol = row.symbol
            fmp_ratings_rating = row.rating
            fmp_ratings_overall_score = row.overallScore
            fmp_ratings_dcf_score = row.discountedCashFlowScore
        }
    } else {
        fmp_ratings_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotLiveDataOK(t, vm, "FMP ratings-snapshot", "fmp_ratings_status", "fmp_ratings_request_error", "fmp_ratings_json_error", "fmp_ratings_ok")
	symbol := mustGetString(t, vm, "fmp_ratings_symbol")
	rating := mustGetString(t, vm, "fmp_ratings_rating")
	overallScore := mustGetFloat(t, vm, "fmp_ratings_overall_score")
	dcfScore := mustGetFloat(t, vm, "fmp_ratings_dcf_score")
	fmt.Printf("fmp_ratings symbol=%q rating=%q overall_score=%f dcf_score=%f\n", symbol, rating, overallScore, dcfScore)
	if symbol != "AAPL" || rating == "" || overallScore <= 0 || dcfScore <= 0 {
		t.Fatalf("unexpected FMP ratings-snapshot payload: symbol=%q rating=%q overall_score=%f dcf_score=%f", symbol, rating, overallScore, dcfScore)
	}
}

func TestFinRobotLiveFMPPriceTargetConsensusDataIntegration(t *testing.T) {
	vm := newFinRobotFMPLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
fmp_target_request_error := nil
fmp_target_json_error := nil
fmp_target_status := 0
fmp_target_ok := false
fmp_target_symbol := ""
fmp_target_low := 0.0
fmp_target_high := 0.0
fmp_target_median := 0.0
fmp_target_consensus := 0.0

url := "https://financialmodelingprep.com/stable/price-target-consensus?symbol=AAPL&apikey=" .. os.getenv("LEIA_FMP_API_KEY")
resp, err := net.get(url, {timeout: 30})
if err != nil {
    fmp_target_request_error = err
} else {
    fmp_target_status = resp.status
    fmp_target_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            fmp_target_json_error = json_err
        } else {
            row := data[1]
            fmp_target_symbol = row.symbol
            fmp_target_low = row.targetLow
            fmp_target_high = row.targetHigh
            fmp_target_median = row.targetMedian
            fmp_target_consensus = row.targetConsensus
        }
    } else {
        fmp_target_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotLiveDataOK(t, vm, "FMP price-target-consensus", "fmp_target_status", "fmp_target_request_error", "fmp_target_json_error", "fmp_target_ok")
	symbol := mustGetString(t, vm, "fmp_target_symbol")
	low := mustGetFloat(t, vm, "fmp_target_low")
	high := mustGetFloat(t, vm, "fmp_target_high")
	median := mustGetFloat(t, vm, "fmp_target_median")
	consensus := mustGetFloat(t, vm, "fmp_target_consensus")
	fmt.Printf("fmp_price_target symbol=%q low=%f high=%f median=%f consensus=%f\n", symbol, low, high, median, consensus)
	if symbol != "AAPL" || low <= 0 || high <= 0 || median <= 0 || consensus <= 0 || low > high || median > high || consensus > high {
		t.Fatalf("unexpected FMP price-target-consensus payload: symbol=%q low=%f high=%f median=%f consensus=%f", symbol, low, high, median, consensus)
	}
}

func TestFinRobotLiveFMPIncomeStatementDataIntegration(t *testing.T) {
	vm := newFinRobotFMPLiveDataVM(t)
	if err := execFinRobotFMPStableRowScript(t, vm, "fmp_income", "income-statement", []finRobotFMPRowField{
		{name: "symbol", init: `""`, expr: "row.symbol"},
		{name: "date", init: `""`, expr: "row.date"},
		{name: "currency", init: `""`, expr: "row.reportedCurrency"},
		{name: "revenue", init: "0.0", expr: "row.revenue"},
		{name: "gross_profit", init: "0.0", expr: "row.grossProfit"},
		{name: "net_income", init: "0.0", expr: "row.netIncome"},
	}); err != nil {
		t.Fatal(err)
	}

	assertFinRobotLiveDataPrefixOK(t, vm, "FMP income-statement", "fmp_income")
	symbol := mustGetString(t, vm, "fmp_income_symbol")
	date := mustGetString(t, vm, "fmp_income_date")
	currency := mustGetString(t, vm, "fmp_income_currency")
	revenue := mustGetFloat(t, vm, "fmp_income_revenue")
	grossProfit := mustGetFloat(t, vm, "fmp_income_gross_profit")
	netIncome := mustGetFloat(t, vm, "fmp_income_net_income")
	fmt.Printf("fmp_income symbol=%q date=%q currency=%q revenue=%f gross_profit=%f net_income=%f\n", symbol, date, currency, revenue, grossProfit, netIncome)
	if symbol != "AAPL" || date == "" || currency == "" || revenue <= 0 || grossProfit <= 0 || netIncome <= 0 || grossProfit > revenue || netIncome > revenue {
		t.Fatalf("unexpected FMP income-statement payload: symbol=%q date=%q currency=%q revenue=%f gross_profit=%f net_income=%f", symbol, date, currency, revenue, grossProfit, netIncome)
	}
}

func TestFinRobotLiveFMPBalanceSheetStatementDataIntegration(t *testing.T) {
	vm := newFinRobotFMPLiveDataVM(t)
	if err := execFinRobotFMPStableRowScript(t, vm, "fmp_balance", "balance-sheet-statement", []finRobotFMPRowField{
		{name: "symbol", init: `""`, expr: "row.symbol"},
		{name: "date", init: `""`, expr: "row.date"},
		{name: "currency", init: `""`, expr: "row.reportedCurrency"},
		{name: "cash", init: "0.0", expr: "row.cashAndCashEquivalents"},
		{name: "assets", init: "0.0", expr: "row.totalAssets"},
		{name: "liabilities", init: "0.0", expr: "row.totalLiabilities"},
		{name: "equity", init: "0.0", expr: "row.totalStockholdersEquity"},
	}); err != nil {
		t.Fatal(err)
	}

	assertFinRobotLiveDataPrefixOK(t, vm, "FMP balance-sheet-statement", "fmp_balance")
	symbol := mustGetString(t, vm, "fmp_balance_symbol")
	date := mustGetString(t, vm, "fmp_balance_date")
	currency := mustGetString(t, vm, "fmp_balance_currency")
	cash := mustGetFloat(t, vm, "fmp_balance_cash")
	assets := mustGetFloat(t, vm, "fmp_balance_assets")
	liabilities := mustGetFloat(t, vm, "fmp_balance_liabilities")
	equity := mustGetFloat(t, vm, "fmp_balance_equity")
	fmt.Printf("fmp_balance symbol=%q date=%q currency=%q cash=%f assets=%f liabilities=%f equity=%f\n", symbol, date, currency, cash, assets, liabilities, equity)
	if symbol != "AAPL" || date == "" || currency == "" || cash < 0 || assets <= 0 || liabilities <= 0 || equity <= 0 || assets < liabilities {
		t.Fatalf("unexpected FMP balance-sheet payload: symbol=%q date=%q currency=%q cash=%f assets=%f liabilities=%f equity=%f", symbol, date, currency, cash, assets, liabilities, equity)
	}
}

func TestFinRobotLiveFMPCashFlowStatementDataIntegration(t *testing.T) {
	vm := newFinRobotFMPLiveDataVM(t)
	if err := execFinRobotFMPStableRowScript(t, vm, "fmp_cashflow", "cash-flow-statement", []finRobotFMPRowField{
		{name: "symbol", init: `""`, expr: "row.symbol"},
		{name: "date", init: `""`, expr: "row.date"},
		{name: "currency", init: `""`, expr: "row.reportedCurrency"},
		{name: "operating", init: "0.0", expr: "row.netCashProvidedByOperatingActivities"},
		{name: "capex", init: "0.0", expr: "row.capitalExpenditure"},
		{name: "free", init: "0.0", expr: "row.freeCashFlow"},
	}); err != nil {
		t.Fatal(err)
	}

	assertFinRobotLiveDataPrefixOK(t, vm, "FMP cash-flow-statement", "fmp_cashflow")
	symbol := mustGetString(t, vm, "fmp_cashflow_symbol")
	date := mustGetString(t, vm, "fmp_cashflow_date")
	currency := mustGetString(t, vm, "fmp_cashflow_currency")
	operatingCashFlow := mustGetFloat(t, vm, "fmp_cashflow_operating")
	capex := mustGetFloat(t, vm, "fmp_cashflow_capex")
	freeCashFlow := mustGetFloat(t, vm, "fmp_cashflow_free")
	fmt.Printf("fmp_cashflow symbol=%q date=%q currency=%q operating_cf=%f capex=%f free_cf=%f\n", symbol, date, currency, operatingCashFlow, capex, freeCashFlow)
	if symbol != "AAPL" || date == "" || currency == "" || operatingCashFlow <= 0 || freeCashFlow <= 0 || freeCashFlow > operatingCashFlow || capex == 0 {
		t.Fatalf("unexpected FMP cash-flow payload: symbol=%q date=%q currency=%q operating_cf=%f capex=%f free_cf=%f", symbol, date, currency, operatingCashFlow, capex, freeCashFlow)
	}
}
