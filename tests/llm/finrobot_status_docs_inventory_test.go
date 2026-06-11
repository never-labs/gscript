package leia_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFinRobotStatusDocsMatchCurrentInventory(t *testing.T) {
	root := repoRoot(t)
	manifest := loadLivePackagePlanManifest(t, root)
	examples := finrobotAggregateRegistryExamples(t, root)

	liveSkeletons := 0
	genericSkeletons := 0
	for _, skeleton := range manifest.LivePackageSkeletons {
		if skeleton.Status == "checked_in_registered_example" {
			liveSkeletons++
		}
		if strings.HasPrefix(skeleton.ID, "generic_") {
			genericSkeletons++
		}
	}
	if liveSkeletons == 0 || genericSkeletons == 0 || genericSkeletons > liveSkeletons {
		t.Fatalf("live skeleton counts = %d generic=%d, want non-empty coherent counts", liveSkeletons, genericSkeletons)
	}

	docs := map[string][]string{
		"examples/ai/finrobot_translation/README.md": {
			fmt.Sprintf("%d runnable", len(examples)),
			fmt.Sprintf("%d directories", liveSkeletons),
			"generic_*",
			"FinRobot-specific language features",
		},
		"examples/ai/finrobot_translation/live_package_plan.md": {
			fmt.Sprintf("%d runnable", len(examples)),
			fmt.Sprintf("%d", liveSkeletons),
			"checked-in live-package",
			"generic_*",
			"reusable generic AI",
		},
	}
	for rel, expected := range docs {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		text := string(data)
		for _, want := range expected {
			if !strings.Contains(text, want) {
				t.Fatalf("%s missing current inventory phrase %q", rel, want)
			}
		}
		for _, stale := range []string{
			"65 runnable",
			"66 runnable",
			"22 checked-in",
			"22 directories",
			"451 files",
			"597 files",
		} {
			if strings.Contains(text, stale) {
				t.Fatalf("%s still contains stale inventory phrase %q", rel, stale)
			}
		}
	}
}
