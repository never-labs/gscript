package leia_test

import (
	"fmt"
	"strings"
	"testing"
)

func TestFinRobotLiveGitHubRepoMetadataDataIntegration(t *testing.T) {
	vm := newFinRobotLiveDataVM(t)
	if err := execFinRobotLiveDataScript(t, vm, `
github_repo_request_error := nil
github_repo_json_error := nil
github_repo_status := 0
github_repo_ok := false
github_repo_content_type := ""
github_repo_rate_limit_remaining := ""
github_repo_id := 0
github_repo_full_name := ""
github_repo_name := ""
github_repo_owner := ""
github_repo_owner_type := ""
github_repo_private := true
github_repo_fork := true
github_repo_archived := true
github_repo_disabled := true
github_repo_visibility := ""
github_repo_html_url := ""
github_repo_api_url := ""
github_repo_description := ""
github_repo_language := ""
github_repo_default_branch := ""
github_repo_license_key := ""
github_repo_license_name := ""
github_repo_stars := 0
github_repo_forks := 0
github_repo_watchers := 0
github_repo_open_issues := 0
github_repo_created_at := ""
github_repo_updated_at := ""
github_repo_pushed_at := ""

headers := {}
headers["User-Agent"] = "Mozilla/5.0 Leia FinRobot live data smoke"
headers["Accept"] = "application/vnd.github+json"

resp, err := net.get("https://api.github.com/repos/langchain-ai/langchain", {
    headers: headers
    timeout: 30
})
if err != nil {
    github_repo_request_error = err
} else {
    github_repo_status = resp.status
    github_repo_ok = resp.ok
    if resp.headers != nil {
        if resp.headers["Content-Type"] != nil {
            github_repo_content_type = resp.headers["Content-Type"]
        }
        if resp.headers["X-RateLimit-Remaining"] != nil {
            github_repo_rate_limit_remaining = resp.headers["X-RateLimit-Remaining"]
        }
    }
    if resp.ok {
        data, json_err := resp.json()
        if json_err != nil {
            github_repo_json_error = json_err
        } else {
            github_repo_id = data.id
            github_repo_full_name = data.full_name
            github_repo_name = data.name
            github_repo_owner = data.owner.login
            github_repo_owner_type = data.owner.type
            github_repo_private = data.private
            github_repo_fork = data.fork
            github_repo_archived = data.archived
            github_repo_disabled = data.disabled
            github_repo_visibility = data.visibility
            github_repo_html_url = data.html_url
            github_repo_api_url = data.url
            github_repo_description = data.description
            github_repo_language = data.language
            github_repo_default_branch = data.default_branch
            if data.license != nil {
                github_repo_license_key = data.license.key
                github_repo_license_name = data.license.name
            }
            github_repo_stars = data.stargazers_count
            github_repo_forks = data.forks_count
            github_repo_watchers = data.watchers_count
            github_repo_open_issues = data.open_issues_count
            github_repo_created_at = data.created_at
            github_repo_updated_at = data.updated_at
            github_repo_pushed_at = data.pushed_at
        }
    } else {
        github_repo_request_error = resp.statusText
    }
}
`); err != nil {
		t.Fatal(err)
	}

	assertFinRobotPublicLiveDataOK(t, vm, "GitHub repo metadata", "github_repo_status", "github_repo_request_error", "github_repo_json_error", "github_repo_ok")
	contentType := mustGetString(t, vm, "github_repo_content_type")
	rateLimitRemaining := mustGetString(t, vm, "github_repo_rate_limit_remaining")
	id := mustGetInt(t, vm, "github_repo_id")
	fullName := mustGetString(t, vm, "github_repo_full_name")
	name := mustGetString(t, vm, "github_repo_name")
	owner := mustGetString(t, vm, "github_repo_owner")
	ownerType := mustGetString(t, vm, "github_repo_owner_type")
	private := mustGetBool(t, vm, "github_repo_private")
	fork := mustGetBool(t, vm, "github_repo_fork")
	archived := mustGetBool(t, vm, "github_repo_archived")
	disabled := mustGetBool(t, vm, "github_repo_disabled")
	visibility := mustGetString(t, vm, "github_repo_visibility")
	htmlURL := mustGetString(t, vm, "github_repo_html_url")
	apiURL := mustGetString(t, vm, "github_repo_api_url")
	description := mustGetString(t, vm, "github_repo_description")
	language := mustGetString(t, vm, "github_repo_language")
	defaultBranch := mustGetString(t, vm, "github_repo_default_branch")
	licenseKey := mustGetString(t, vm, "github_repo_license_key")
	licenseName := mustGetString(t, vm, "github_repo_license_name")
	stars := mustGetInt(t, vm, "github_repo_stars")
	forks := mustGetInt(t, vm, "github_repo_forks")
	watchers := mustGetInt(t, vm, "github_repo_watchers")
	openIssues := mustGetInt(t, vm, "github_repo_open_issues")
	createdAt := mustGetString(t, vm, "github_repo_created_at")
	updatedAt := mustGetString(t, vm, "github_repo_updated_at")
	pushedAt := mustGetString(t, vm, "github_repo_pushed_at")

	fmt.Printf("github_repo full_name=%q owner=%q stars=%d forks=%d watchers=%d open_issues=%d language=%q license=%q/%q branch=%q rate_limit_remaining=%q\n", fullName, owner, stars, forks, watchers, openIssues, language, licenseKey, licenseName, defaultBranch, rateLimitRemaining)
	if contentType == "" || !strings.Contains(contentType, "application/json") {
		t.Fatalf("GitHub repo Content-Type = %q, want JSON", contentType)
	}
	if id <= 0 || fullName != "langchain-ai/langchain" || name != "langchain" || owner != "langchain-ai" || ownerType != "Organization" {
		t.Fatalf("unexpected GitHub repo identity: id=%d full_name=%q name=%q owner=%q owner_type=%q", id, fullName, name, owner, ownerType)
	}
	if private || fork || archived || disabled || visibility != "public" {
		t.Fatalf("unexpected GitHub repo flags: private=%t fork=%t archived=%t disabled=%t visibility=%q", private, fork, archived, disabled, visibility)
	}
	if htmlURL != "https://github.com/langchain-ai/langchain" || !strings.HasPrefix(apiURL, "https://api.github.com/repos/langchain-ai/langchain") {
		t.Fatalf("unexpected GitHub repo URLs: html=%q api=%q", htmlURL, apiURL)
	}
	if !strings.Contains(strings.ToLower(description), "agent") || language == "" || defaultBranch == "" || licenseKey == "" || licenseName == "" {
		t.Fatalf("unexpected GitHub repo metadata: description=%q language=%q branch=%q license=%q/%q", description, language, defaultBranch, licenseKey, licenseName)
	}
	if stars < 10000 || forks <= 0 || watchers != stars || openIssues < 0 {
		t.Fatalf("unexpected GitHub repo activity metrics: stars=%d forks=%d watchers=%d open_issues=%d", stars, forks, watchers, openIssues)
	}
	if createdAt == "" || updatedAt == "" || pushedAt == "" {
		t.Fatalf("unexpected GitHub repo timestamps: created=%q updated=%q pushed=%q", createdAt, updatedAt, pushedAt)
	}
}
