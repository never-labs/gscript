package tests_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGoStyleConcurrencyContract(t *testing.T) {
	root := findRepoRoot(t)
	cases := []struct {
		name     string
		rel      string
		wantText []string
	}{
		{
			name:     "language_go_channel_host",
			rel:      filepath.Join("tests", "language", "go_channel_host_more.leia"),
			wantText: []string{"case:go_channel_host_more", "ok"},
		},
		{
			name:     "language_go_channel_edges",
			rel:      filepath.Join("tests", "language", "go_channel_edges_more.leia"),
			wantText: []string{"case:go_channel_edges_more", "ok"},
		},
		{
			name:     "example_select_timeout",
			rel:      filepath.Join("examples", "concurrency", "select_timeout.leia"),
			wantText: []string{"timeout"},
		},
		{
			name:     "example_context_sleep_cancel",
			rel:      filepath.Join("examples", "concurrency", "context_sleep.leia"),
			wantText: []string{"ok=false", "err=deadline exceeded", "elapsed_lt_full=true"},
		},
		{
			name:     "example_sync_group_cancel",
			rel:      filepath.Join("examples", "concurrency", "sync_group_cancel.leia"),
			wantText: []string{"ok=false", "failures=1", "sleep_ok=false", "err=cancelled"},
		},
		{
			name:     "example_sync_group",
			rel:      filepath.Join("examples", "concurrency", "sync_group.leia"),
			wantText: []string{"sum=30", "failures=0"},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			out := runLeiaFile(t, filepath.Join(root, tc.rel))
			for _, want := range tc.wantText {
				if !strings.Contains(out, want) {
					t.Fatalf("%s output = %q, want containing %q", tc.rel, out, want)
				}
			}
		})
	}
}
