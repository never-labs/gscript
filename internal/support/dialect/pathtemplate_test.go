package dialect

import "testing"

func TestPathTemplateMatchAndExpand(t *testing.T) {
	match, err := MatchPathTemplate("/v1/users/{id}/files/{*rest}", "/v1/users/alice%40example/files/docs/report%201.pdf")
	if err != nil {
		t.Fatalf("MatchPathTemplate: %v", err)
	}
	if !match.Matched {
		t.Fatal("matched = false, want true")
	}
	if got := match.Params["id"]; got != "alice@example" {
		t.Fatalf("id = %q, want alice@example", got)
	}
	if got := match.Params["rest"]; got != "docs/report 1.pdf" {
		t.Fatalf("rest = %q, want docs/report 1.pdf", got)
	}

	path, err := ExpandPathTemplate("/v1/users/{id}/files/{*rest}", map[string]string{
		"id":   "alice@example",
		"rest": "docs/report 1.pdf",
	})
	if err != nil {
		t.Fatalf("ExpandPathTemplate: %v", err)
	}
	if want := "/v1/users/alice@example/files/docs/report%201.pdf"; path != want {
		t.Fatalf("expanded path = %q, want %q", path, want)
	}
}

func TestPathTemplateNoMatchAndValidation(t *testing.T) {
	match, err := MatchPathTemplate("/v1/users/{id}", "/v1/orgs/123")
	if err != nil {
		t.Fatalf("MatchPathTemplate: %v", err)
	}
	if match.Matched {
		t.Fatal("matched = true, want false")
	}

	for _, template := range []string{
		"v1/users/{id}",
		"/v1/{bad-name}",
		"/v1/{id}/{id}",
		"/v1/{*rest}/tail",
	} {
		if _, err := MatchPathTemplate(template, "/v1/users/123"); err == nil {
			t.Fatalf("MatchPathTemplate(%q) succeeded, want error", template)
		}
	}

	if _, err := ExpandPathTemplate("/v1/users/{id}", map[string]string{}); err == nil {
		t.Fatal("ExpandPathTemplate succeeded without id, want error")
	}
	if _, err := ExpandPathTemplate("/v1/users/{id}", map[string]string{"id": "a/b"}); err == nil {
		t.Fatal("ExpandPathTemplate accepted slash in single segment, want error")
	}
}
