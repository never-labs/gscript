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
	providerFreeFixtureIndexes := 0
	for _, skeleton := range manifest.LivePackageSkeletons {
		if skeleton.Status == "checked_in_registered_example" {
			liveSkeletons++
		}
		if strings.HasPrefix(skeleton.ID, "generic_") {
			genericSkeletons++
		}
	}
	fixtureIndexMatches, err := filepath.Glob(filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages", "*", "fixtures", "provider_free_fixture_index.json"))
	if err != nil {
		t.Fatal(err)
	}
	providerFreeFixtureIndexes = len(fixtureIndexMatches)
	if liveSkeletons == 0 || genericSkeletons == 0 || genericSkeletons > liveSkeletons {
		t.Fatalf("live skeleton counts = %d generic=%d, want non-empty coherent counts", liveSkeletons, genericSkeletons)
	}
	files := finrobotTranslationFileCount(t, root)

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
		"examples/ai/finrobot_translation/COVERAGE.md": {
			fmt.Sprintf("%d files", files),
			fmt.Sprintf("%d\n  provider-free live-package skeleton directories", liveSkeletons),
			fmt.Sprintf("%d live-package skeletons use", providerFreeFixtureIndexes),
			fmt.Sprintf("`%d` registered generic AI live-package examples", genericSkeletons),
		},
		"examples/ai/finrobot_translation/VERIFICATION.md": {
			fmt.Sprintf("%d files", files),
			fmt.Sprintf("`%d` registered live-package examples", liveSkeletons),
			fmt.Sprintf("`%d` registered generic AI live-package examples", genericSkeletons),
			fmt.Sprintf("`%d` provider-free fixture indexes", providerFreeFixtureIndexes),
		},
		"examples/ai/finrobot_translation/GAPS.md": {
			fmt.Sprintf("%d registered examples", len(examples)),
			fmt.Sprintf("%d checked-in live-package skeleton directories", liveSkeletons),
			"no open generic AI dialect gap",
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
			"630 files",
			"633 files",
			"635 files",
			"32 live-package skeletons use",
		} {
			if strings.Contains(text, stale) {
				t.Fatalf("%s still contains stale inventory phrase %q", rel, stale)
			}
		}
	}
}

func finrobotTranslationFileCount(t *testing.T, root string) int {
	t.Helper()
	count := 0
	base := filepath.Join(root, "examples", "ai", "finrobot_translation")
	if err := filepath.WalkDir(base, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			count++
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return count
}
