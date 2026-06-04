package dialect

import (
	"strings"
	"testing"
)

func TestParseJUnitSuitesSummary(t *testing.T) {
	report, err := ParseJUnit(`<testsuites name="ci" tests="3" failures="1" errors="0" skipped="1" time="1.25">
  <testsuite name="unit" tests="2" failures="1" errors="0" skipped="0" time="0.75">
    <testcase classname="pkg.A" name="passes" time="0.10"/>
    <testcase classname="pkg.A" name="fails" time="0.20">
      <failure type="assert" message="want true">stack line</failure>
    </testcase>
  </testsuite>
  <testsuite name="integration" tests="1" failures="0" errors="0" skipped="1" time="0.50">
    <testcase classname="pkg.B" name="skips"><skipped message="not configured"/></testcase>
  </testsuite>
</testsuites>`)
	if err != nil {
		t.Fatalf("ParseJUnit: %v", err)
	}
	if report.Name != "ci" || report.Tests != 3 || report.Failures != 1 || report.Errors != 0 || report.Skipped != 1 || report.Passed != 1 {
		t.Fatalf("report summary = %+v", report)
	}
	if report.Time != 1.25 {
		t.Fatalf("report time = %v, want 1.25", report.Time)
	}
	if len(report.Suites) != 2 || len(report.Cases) != 3 {
		t.Fatalf("suite/case counts = %d/%d", len(report.Suites), len(report.Cases))
	}
	if got := report.Suites[0].Cases[1]; got.Status != "failed" || got.Message != "want true" || got.Type != "assert" || got.Text != "stack line" {
		t.Fatalf("failed case = %+v", got)
	}
	if got := report.Cases[2]; got.Status != "skipped" || got.Message != "not configured" {
		t.Fatalf("skipped case = %+v", got)
	}
}

func TestParseJUnitSingleSuiteComputesTotals(t *testing.T) {
	report, err := ParseJUnit(`<testsuite name="unit">
  <testcase classname="pkg.A" name="passes" time="0.10"/>
  <testcase classname="pkg.A" name="errors"><error>boom</error></testcase>
</testsuite>`)
	if err != nil {
		t.Fatalf("ParseJUnit: %v", err)
	}
	if report.Tests != 2 || report.Errors != 1 || report.Passed != 1 || len(report.Suites) != 1 {
		t.Fatalf("report = %+v", report)
	}
	if got := report.Cases[1]; got.Status != "error" || got.Message != "boom" {
		t.Fatalf("error case = %+v", got)
	}
}

func TestParseJUnitErrors(t *testing.T) {
	_, err := ParseJUnit(`<testsuite tests="nope"/>`)
	if err == nil || !strings.Contains(err.Error(), `junit dialect: testsuite 1: invalid tests attribute "nope"`) {
		t.Fatalf("ParseJUnit attr error = %v", err)
	}
	_, err = ParseJUnit(`<testsuites><not-suite/></testsuites>`)
	if err != nil {
		t.Fatalf("unexpected empty testsuites error: %v", err)
	}
	_, err = ParseJUnit(`<report/>`)
	if err == nil || !strings.Contains(err.Error(), `root element must be testsuite or testsuites`) {
		t.Fatalf("ParseJUnit root error = %v", err)
	}
}
