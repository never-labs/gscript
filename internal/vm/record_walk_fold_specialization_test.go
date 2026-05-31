package vm

import (
	"math"
	"strings"
	"testing"

	"github.com/never-labs/gscript/internal/runtime"
)

func TestRecordWalkFoldSpecializationDerivesFieldsFromBytecode(t *testing.T) {
	const src = `
func make_row(i) {
    return {
        ident: i,
        flavor: "article",
        live: i % 4 != 0,
        account: {level: i % 6, zone: (i % 9) + 1},
        counters: {seen: i * 3 % 1000, taps: i * 7 % 251, faults: i % 13},
        labels: {alpha: "aa", beta: "bbb", gamma: "cccc"}
    }
}
func build(n) {
    rows := {}
    for i := 1; i <= n; i++ { rows[i] = make_row(i) }
    return rows
}
func fold_rows(rows, n, passes) {
    checksum := 0
    for pass := 1; pass <= passes; pass++ {
        for i := 1; i <= n; i++ {
            row := rows[i]
            metrics := row.counters
            user := row.account
            tags := row.labels
            tag_choice := (i + pass) % 3
            tag := tags.alpha
            if tag_choice == 1 {
                tag = tags.beta
            } elseif tag_choice == 2 {
                tag = tags.gamma
            }
            if row.live {
                score := metrics.seen + metrics.taps * 3 - metrics.faults * 5 + user.level + #row.flavor + #tag
                checksum = (checksum + score + user.zone) % 1000000007
                if pass % 3 == 0 {
                    metrics.seen = (metrics.seen + user.level + i) % 2000
                }
            } else {
                checksum = (checksum + row.ident + metrics.faults + #tags.alpha) % 1000000007
            }
        }
    }
    return checksum
}
`
	proto, vm := compileSpectralSpecializationTestProgram(t, src)
	defer vm.Close()
	if _, err := vm.Execute(proto); err != nil {
		t.Fatalf("execute definitions: %v", err)
	}
	fold := findTestProtoByName(proto, "fold_rows")
	if fold != nil {
		fold.Name = "shape_only_record_fold"
		fold.Source = "host/generated/not-a-benchmark.gs"
	}
	if fold == nil || !isRecordWalkFoldProto(fold) {
		t.Fatal("renamed record fold proto not recognized")
	}
	spec, ok := recordWalkFoldSpecForProto(fold)
	if !ok {
		t.Fatal("record fold spec not derived")
	}
	if !cachedRuntimeSpecializationRecognized(fold, runtimeSpecializationRecordWalkFold) {
		t.Fatal("record_walk_fold rejected by runtime specialization cache")
	}
	if fold.RecordWalkFoldSpecialization == nil || fold.RecordWalkFoldSpecialization.spec == nil {
		t.Fatal("record_walk_fold proto-local spec was not generated")
	}
	if spec.recordFields != [6]string{"ident", "flavor", "live", "account", "counters", "labels"} ||
		spec.metricFields != [3]string{"seen", "taps", "faults"} ||
		spec.userFields != [2]string{"level", "zone"} ||
		spec.tagFields != [3]string{"alpha", "beta", "gamma"} {
		t.Fatalf("unexpected spec: %+v", spec)
	}

	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()
	specializationGlobals := compileAndRun(t, src+`
rows := build(64)
result := fold_rows(rows, 64, 5)
seen1 := rows[1].counters.seen
`)
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteCallSiteValue, "record_walk_fold"); got != 1 {
		t.Fatalf("record_walk_fold structural hit count = %d, want 1", got)
	}
	fallbackSrc := strings.Replace(src, `
    return checksum
}`, `
    noop := 0
    checksum = checksum + noop
    return checksum
}`, 1)
	fallbackGlobals := compileAndRun(t, fallbackSrc+`
rows := build(64)
result := fold_rows(rows, 64, 5)
seen1 := rows[1].counters.seen
`)
	got := specializationGlobals["result"].Number()
	want := fallbackGlobals["result"].Number()
	if math.Abs(got-want) > 0 {
		t.Fatalf("specialization result %.0f, fallback %.0f", got, want)
	}
	gotSeen := specializationGlobals["seen1"].Number()
	wantSeen := fallbackGlobals["seen1"].Number()
	if math.Abs(gotSeen-wantSeen) > 0 {
		t.Fatalf("mutated metric %.0f, fallback %.0f", gotSeen, wantSeen)
	}
}
