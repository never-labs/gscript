package dialect

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

type JUnitReport struct {
	Name     string
	Tests    int
	Failures int
	Errors   int
	Skipped  int
	Passed   int
	Time     float64
	Suites   []JUnitSuite
	Cases    []JUnitCase
}

type JUnitSuite struct {
	Name     string
	Tests    int
	Failures int
	Errors   int
	Skipped  int
	Passed   int
	Time     float64
	Cases    []JUnitCase
}

type JUnitCase struct {
	Name      string
	ClassName string
	Time      float64
	Status    string
	Message   string
	Type      string
	Text      string
}

type junitXMLRoot struct {
	XMLName   xml.Name
	Name      string          `xml:"name,attr"`
	Tests     string          `xml:"tests,attr"`
	Failures  string          `xml:"failures,attr"`
	Errors    string          `xml:"errors,attr"`
	Skipped   string          `xml:"skipped,attr"`
	Time      string          `xml:"time,attr"`
	Suites    []junitXMLSuite `xml:"testsuite"`
	TestCases []junitXMLCase  `xml:"testcase"`
}

type junitXMLSuite struct {
	Name      string         `xml:"name,attr"`
	Tests     string         `xml:"tests,attr"`
	Failures  string         `xml:"failures,attr"`
	Errors    string         `xml:"errors,attr"`
	Skipped   string         `xml:"skipped,attr"`
	Time      string         `xml:"time,attr"`
	TestCases []junitXMLCase `xml:"testcase"`
}

type junitXMLCase struct {
	Name      string             `xml:"name,attr"`
	ClassName string             `xml:"classname,attr"`
	Time      string             `xml:"time,attr"`
	Failures  []junitXMLCaseNode `xml:"failure"`
	Errors    []junitXMLCaseNode `xml:"error"`
	Skipped   []junitXMLCaseNode `xml:"skipped"`
}

type junitXMLCaseNode struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Text    string `xml:",chardata"`
}

func ParseJUnit(src string) (JUnitReport, error) {
	var root junitXMLRoot
	if err := xml.Unmarshal([]byte(src), &root); err != nil {
		return JUnitReport{}, &ParseError{Kind: "junit", Message: err.Error()}
	}
	switch root.XMLName.Local {
	case "testsuites":
		return junitReportFromRoot(root)
	case "testsuite":
		suiteXML := junitXMLSuite{
			Name:      root.Name,
			Tests:     root.Tests,
			Failures:  root.Failures,
			Errors:    root.Errors,
			Skipped:   root.Skipped,
			Time:      root.Time,
			TestCases: root.TestCases,
		}
		suite, err := junitSuiteFromXML(suiteXML, 1)
		if err != nil {
			return JUnitReport{}, err
		}
		return junitReportFromSuites(root, []JUnitSuite{suite})
	default:
		return JUnitReport{}, &ParseError{Kind: "junit", Message: fmt.Sprintf("root element must be testsuite or testsuites, got %q", root.XMLName.Local)}
	}
}

func junitReportFromRoot(root junitXMLRoot) (JUnitReport, error) {
	suites := make([]JUnitSuite, 0, len(root.Suites))
	for i, suiteXML := range root.Suites {
		suite, err := junitSuiteFromXML(suiteXML, i+1)
		if err != nil {
			return JUnitReport{}, err
		}
		suites = append(suites, suite)
	}
	return junitReportFromSuites(root, suites)
}

func junitReportFromSuites(root junitXMLRoot, suites []JUnitSuite) (JUnitReport, error) {
	report := JUnitReport{Name: root.Name, Suites: suites}
	for _, suite := range suites {
		report.Tests += suite.Tests
		report.Failures += suite.Failures
		report.Errors += suite.Errors
		report.Skipped += suite.Skipped
		report.Time += suite.Time
		report.Cases = append(report.Cases, suite.Cases...)
	}
	if root.Tests != "" {
		n, err := junitIntAttr("testsuites", "tests", root.Tests)
		if err != nil {
			return JUnitReport{}, err
		}
		report.Tests = n
	}
	if root.Failures != "" {
		n, err := junitIntAttr("testsuites", "failures", root.Failures)
		if err != nil {
			return JUnitReport{}, err
		}
		report.Failures = n
	}
	if root.Errors != "" {
		n, err := junitIntAttr("testsuites", "errors", root.Errors)
		if err != nil {
			return JUnitReport{}, err
		}
		report.Errors = n
	}
	if root.Skipped != "" {
		n, err := junitIntAttr("testsuites", "skipped", root.Skipped)
		if err != nil {
			return JUnitReport{}, err
		}
		report.Skipped = n
	}
	if root.Time != "" {
		n, err := junitFloatAttr("testsuites", "time", root.Time)
		if err != nil {
			return JUnitReport{}, err
		}
		report.Time = n
	}
	report.Passed = report.Tests - report.Failures - report.Errors - report.Skipped
	if report.Passed < 0 {
		report.Passed = 0
	}
	return report, nil
}

func junitSuiteFromXML(src junitXMLSuite, index int) (JUnitSuite, error) {
	where := fmt.Sprintf("testsuite %d", index)
	suite := JUnitSuite{Name: src.Name, Cases: make([]JUnitCase, 0, len(src.TestCases))}
	for i, caseXML := range src.TestCases {
		tc, err := junitCaseFromXML(caseXML, i+1)
		if err != nil {
			return JUnitSuite{}, err
		}
		suite.Cases = append(suite.Cases, tc)
		switch tc.Status {
		case "failed":
			suite.Failures++
		case "error":
			suite.Errors++
		case "skipped":
			suite.Skipped++
		}
		suite.Time += tc.Time
	}
	suite.Tests = len(suite.Cases)
	if src.Tests != "" {
		n, err := junitIntAttr(where, "tests", src.Tests)
		if err != nil {
			return JUnitSuite{}, err
		}
		suite.Tests = n
	}
	if src.Failures != "" {
		n, err := junitIntAttr(where, "failures", src.Failures)
		if err != nil {
			return JUnitSuite{}, err
		}
		suite.Failures = n
	}
	if src.Errors != "" {
		n, err := junitIntAttr(where, "errors", src.Errors)
		if err != nil {
			return JUnitSuite{}, err
		}
		suite.Errors = n
	}
	if src.Skipped != "" {
		n, err := junitIntAttr(where, "skipped", src.Skipped)
		if err != nil {
			return JUnitSuite{}, err
		}
		suite.Skipped = n
	}
	if src.Time != "" {
		n, err := junitFloatAttr(where, "time", src.Time)
		if err != nil {
			return JUnitSuite{}, err
		}
		suite.Time = n
	}
	suite.Passed = suite.Tests - suite.Failures - suite.Errors - suite.Skipped
	if suite.Passed < 0 {
		suite.Passed = 0
	}
	return suite, nil
}

func junitCaseFromXML(src junitXMLCase, index int) (JUnitCase, error) {
	tc := JUnitCase{Name: src.Name, ClassName: src.ClassName, Status: "passed"}
	if src.Time != "" {
		n, err := junitFloatAttr(fmt.Sprintf("testcase %d", index), "time", src.Time)
		if err != nil {
			return JUnitCase{}, err
		}
		tc.Time = n
	}
	if len(src.Errors) > 0 {
		tc.Status = "error"
		junitApplyCaseNode(&tc, src.Errors[0])
		return tc, nil
	}
	if len(src.Failures) > 0 {
		tc.Status = "failed"
		junitApplyCaseNode(&tc, src.Failures[0])
		return tc, nil
	}
	if len(src.Skipped) > 0 {
		tc.Status = "skipped"
		junitApplyCaseNode(&tc, src.Skipped[0])
	}
	return tc, nil
}

func junitApplyCaseNode(tc *JUnitCase, node junitXMLCaseNode) {
	tc.Message = strings.TrimSpace(node.Message)
	tc.Type = strings.TrimSpace(node.Type)
	tc.Text = strings.TrimSpace(node.Text)
	if tc.Message == "" {
		tc.Message = tc.Text
	}
}

func junitIntAttr(where, name, raw string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 0 {
		return 0, &ParseError{Kind: "junit", Message: fmt.Sprintf("%s: invalid %s attribute %q", where, name, raw)}
	}
	return n, nil
}

func junitFloatAttr(where, name, raw string) (float64, error) {
	n, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || n < 0 {
		return 0, &ParseError{Kind: "junit", Message: fmt.Sprintf("%s: invalid %s attribute %q", where, name, raw)}
	}
	return n, nil
}
