package leia_test

import (
	"fmt"
	"strings"
	"testing"
)

func TestFinRobotLiveNISTNVDCVEDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
nist_nvd_cve_request_error := nil
nist_nvd_cve_json_error := nil
nist_nvd_cve_status := 0
nist_nvd_cve_ok := false
nist_nvd_cve_total := 0
nist_nvd_cve_count := 0
nist_nvd_cve_id := ""
nist_nvd_cve_published := ""
nist_nvd_cve_last_modified := ""
nist_nvd_cve_description := ""
nist_nvd_cve_cvss_severity := ""
nist_nvd_cve_cvss_score := 0.0

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke contact=opensource@example.invalid"
headers["Accept"] = "application/json"

resp, err := net.get("https://services.nvd.nist.gov/rest/json/cves/2.0?keywordSearch=openssl&cvssV3Severity=HIGH&resultsPerPage=1", {
    headers: headers
    timeout: 30
})
if err != nil {
    nist_nvd_cve_request_error = err
} else {
    nist_nvd_cve_status = resp.status
    nist_nvd_cve_ok = resp.ok
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            nist_nvd_cve_json_error = json_err
        } else {
            nist_nvd_cve_total = data.totalResults
            nist_nvd_cve_count = #data.vulnerabilities
            if nist_nvd_cve_count > 0 {
                vuln := data.vulnerabilities[1]
                cve := vuln.cve
                nist_nvd_cve_id = cve.id
                nist_nvd_cve_published = cve.published
                nist_nvd_cve_last_modified = cve.lastModified
                if cve.descriptions != nil && #cve.descriptions > 0 {
                    nist_nvd_cve_description = cve.descriptions[1].value
                }
                if cve.metrics != nil {
                    if cve.metrics.cvssMetricV31 != nil && #cve.metrics.cvssMetricV31 > 0 {
                        metric := cve.metrics.cvssMetricV31[1].cvssData
                        nist_nvd_cve_cvss_severity = metric.baseSeverity
                        nist_nvd_cve_cvss_score = metric.baseScore
                    } else if cve.metrics.cvssMetricV30 != nil && #cve.metrics.cvssMetricV30 > 0 {
                        metric := cve.metrics.cvssMetricV30[1].cvssData
                        nist_nvd_cve_cvss_severity = metric.baseSeverity
                        nist_nvd_cve_cvss_score = metric.baseScore
                    }
                }
            }
        }
    } else {
        nist_nvd_cve_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "NIST NVD CVE 2.0", "nist_nvd_cve_status", "nist_nvd_cve_request_error", "nist_nvd_cve_json_error", "nist_nvd_cve_ok")
	total := mustGetInt(t, vm, "nist_nvd_cve_total")
	count := mustGetInt(t, vm, "nist_nvd_cve_count")
	cveID := mustGetString(t, vm, "nist_nvd_cve_id")
	published := mustGetString(t, vm, "nist_nvd_cve_published")
	lastModified := mustGetString(t, vm, "nist_nvd_cve_last_modified")
	description := mustGetString(t, vm, "nist_nvd_cve_description")
	severity := mustGetString(t, vm, "nist_nvd_cve_cvss_severity")
	score := mustGetFloat(t, vm, "nist_nvd_cve_cvss_score")

	fmt.Printf("nist_nvd_cve total=%d count=%d id=%q published=%q last_modified=%q severity=%q score=%f\n", total, count, cveID, published, lastModified, severity, score)
	if total <= 0 || count <= 0 {
		t.Fatalf("unexpected NIST NVD CVE result counts: total=%d count=%d", total, count)
	}
	if !strings.HasPrefix(cveID, "CVE-") || published == "" || lastModified == "" || description == "" {
		t.Fatalf("unexpected NIST NVD CVE payload: id=%q published=%q last_modified=%q description_empty=%t", cveID, published, lastModified, description == "")
	}
	if severity != "" || score != 0 {
		if !isNISTNVDCVSSV3Severity(severity) || score < 0 || score > 10 {
			t.Fatalf("unexpected NIST NVD CVSS metadata: severity=%q score=%f", severity, score)
		}
	}
}

func isNISTNVDCVSSV3Severity(severity string) bool {
	switch severity {
	case "LOW", "MEDIUM", "HIGH", "CRITICAL":
		return true
	default:
		return false
	}
}
