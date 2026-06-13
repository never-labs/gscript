package leia_test

import (
	"fmt"
	"strings"
	"testing"
)

func TestFinRobotLivePyPIPackageMetadataDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
pypi_package_request_error := nil
pypi_package_json_error := nil
pypi_package_status := 0
pypi_package_ok := false
pypi_package_content_type := ""
pypi_package_name := ""
pypi_package_version := ""
pypi_package_summary := ""
pypi_package_license := ""
pypi_package_requires_python := ""
pypi_package_author := ""
pypi_package_classifier_count := 0
pypi_package_ai_classifier := false
pypi_package_python_classifier := false
pypi_package_release_count := 0
pypi_package_last_serial := 0
pypi_package_url_count := 0
pypi_package_first_filename := ""
pypi_package_first_type := ""
pypi_package_first_python_version := ""
pypi_package_first_requires_python := ""
pypi_package_first_size := 0
pypi_package_first_yanked := true
pypi_package_homepage := ""
pypi_package_repository := ""
pypi_package_documentation := ""
pypi_package_issues := ""

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "application/json"

resp, err := net.get("https://pypi.org/pypi/langchain/json", {
    headers: headers
    timeout: 30
})
if err != nil {
    pypi_package_request_error = err
} else {
    pypi_package_status = resp.status
    pypi_package_ok = resp.ok
    if resp.headers != nil && resp.headers["Content-Type"] != nil {
        pypi_package_content_type = resp.headers["Content-Type"]
    }
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            pypi_package_json_error = json_err
        } else {
            pypi_package_name = data.info.name
            pypi_package_version = data.info.version
            pypi_package_summary = data.info.summary
            pypi_package_license = data.info.license
            pypi_package_requires_python = data.info.requires_python
            if data.info.author != nil {
                pypi_package_author = data.info.author
            }
            if data.info.classifiers != nil {
                pypi_package_classifier_count = #data.info.classifiers
                for _, classifier := range pairs(data.info.classifiers) {
                    if string.contains(classifier, "Artificial Intelligence") {
                        pypi_package_ai_classifier = true
                    }
                    if string.contains(classifier, "Programming Language :: Python :: 3") {
                        pypi_package_python_classifier = true
                    }
                }
            }
            if data.releases != nil {
                pypi_package_release_count = #data.releases
            }
            pypi_package_last_serial = data.last_serial
            if data.urls != nil {
                pypi_package_url_count = #data.urls
                if pypi_package_url_count > 0 {
                    file := data.urls[1]
                    pypi_package_first_filename = file.filename
                    pypi_package_first_type = file.packagetype
                    pypi_package_first_python_version = file.python_version
                    pypi_package_first_requires_python = file.requires_python
                    pypi_package_first_size = file.size
                    pypi_package_first_yanked = file.yanked
                }
            }
            if data.info.project_urls != nil {
                pypi_package_homepage = data.info.project_urls.Homepage
                pypi_package_repository = data.info.project_urls.Repository
                pypi_package_documentation = data.info.project_urls.Documentation
                pypi_package_issues = data.info.project_urls.Issues
            }
        }
    } else {
        pypi_package_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "PyPI package metadata", "pypi_package_status", "pypi_package_request_error", "pypi_package_json_error", "pypi_package_ok")
	contentType := mustGetString(t, vm, "pypi_package_content_type")
	name := mustGetString(t, vm, "pypi_package_name")
	version := mustGetString(t, vm, "pypi_package_version")
	summary := mustGetString(t, vm, "pypi_package_summary")
	license := mustGetString(t, vm, "pypi_package_license")
	requiresPython := mustGetString(t, vm, "pypi_package_requires_python")
	classifierCount := mustGetInt(t, vm, "pypi_package_classifier_count")
	aiClassifier := mustGetBool(t, vm, "pypi_package_ai_classifier")
	pythonClassifier := mustGetBool(t, vm, "pypi_package_python_classifier")
	releaseCount := mustGetInt(t, vm, "pypi_package_release_count")
	lastSerial := mustGetInt(t, vm, "pypi_package_last_serial")
	urlCount := mustGetInt(t, vm, "pypi_package_url_count")
	firstFilename := mustGetString(t, vm, "pypi_package_first_filename")
	firstType := mustGetString(t, vm, "pypi_package_first_type")
	firstPythonVersion := mustGetString(t, vm, "pypi_package_first_python_version")
	firstRequiresPython := mustGetString(t, vm, "pypi_package_first_requires_python")
	firstSize := mustGetInt(t, vm, "pypi_package_first_size")
	firstYanked := mustGetBool(t, vm, "pypi_package_first_yanked")
	homepage := mustGetString(t, vm, "pypi_package_homepage")
	repository := mustGetString(t, vm, "pypi_package_repository")
	documentation := mustGetString(t, vm, "pypi_package_documentation")
	issues := mustGetString(t, vm, "pypi_package_issues")

	fmt.Printf("pypi_package name=%q version=%q license=%q requires_python=%q releases=%d urls=%d first=%q/%q serial=%d\n", name, version, license, requiresPython, releaseCount, urlCount, firstFilename, firstType, lastSerial)
	if contentType == "" || !strings.Contains(contentType, "application/json") {
		t.Fatalf("PyPI Content-Type = %q, want JSON", contentType)
	}
	lowerSummary := strings.ToLower(summary)
	if name != "langchain" || version == "" || (!strings.Contains(lowerSummary, "llm") && !strings.Contains(lowerSummary, "language model")) {
		t.Fatalf("unexpected PyPI package identity: name=%q version=%q summary=%q", name, version, summary)
	}
	if license != "MIT" || !strings.Contains(requiresPython, ">=3.10") {
		t.Fatalf("unexpected PyPI package compatibility metadata: license=%q requires_python=%q", license, requiresPython)
	}
	if classifierCount <= 0 || !aiClassifier || !pythonClassifier {
		t.Fatalf("unexpected PyPI classifiers: count=%d ai=%t python=%t", classifierCount, aiClassifier, pythonClassifier)
	}
	if releaseCount < 0 || lastSerial <= 0 || urlCount <= 0 {
		t.Fatalf("unexpected PyPI release metadata: releases=%d last_serial=%d urls=%d", releaseCount, lastSerial, urlCount)
	}
	if firstFilename == "" || firstType == "" || firstPythonVersion == "" || !strings.Contains(firstRequiresPython, ">=3.10") || firstSize <= 0 || firstYanked {
		t.Fatalf("unexpected PyPI distribution metadata: filename=%q type=%q pyver=%q requires=%q size=%d yanked=%t", firstFilename, firstType, firstPythonVersion, firstRequiresPython, firstSize, firstYanked)
	}
	if !strings.HasPrefix(homepage, "https://") || !strings.HasPrefix(repository, "https://github.com/langchain-ai/") || !strings.HasPrefix(documentation, "https://") || !strings.HasPrefix(issues, "https://github.com/langchain-ai/") {
		t.Fatalf("unexpected PyPI project URLs: homepage=%q repository=%q docs=%q issues=%q", homepage, repository, documentation, issues)
	}
}
