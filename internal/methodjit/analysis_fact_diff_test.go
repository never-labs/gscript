package methodjit

import "testing"

func TestSnapshotAnalysisResultDomainsPreservesTopLevelMapSnapshots(t *testing.T) {
	a := NewAnalysisResult()
	a.Numeric.Int48Safe[42] = true

	snapshots := snapshotAnalysisResultDomains(a)
	snapshot, ok := snapshots["Numeric.Int48Safe"]
	if !ok {
		t.Fatalf("snapshot missing domain map Numeric.Int48Safe")
	}
	if snapshot.Count != 1 || snapshot.Hash == 0 {
		t.Fatalf("Numeric.Int48Safe snapshot = %+v, want count 1 with nonzero hash", snapshot)
	}
}

func TestSnapshotAnalysisResultDomainsIncludesDomainStructMaps(t *testing.T) {
	a := &AnalysisResult{
		Call: &CallFacts{
			CallABIs: map[int]CallABIDescriptor{
				7: {NumArgs: 2, NumRets: 1},
			},
		},
	}

	snapshots := snapshotAnalysisResultDomains(a)
	snapshot, ok := snapshots["Call.CallABIs"]
	if !ok {
		t.Fatalf("snapshot missing domain map Call.CallABIs")
	}
	if snapshot.Count != 1 || snapshot.Hash == 0 {
		t.Fatalf("Call.CallABIs snapshot = %+v, want count 1 with nonzero hash", snapshot)
	}
}

func TestDiffAnalysisFactDomainsReportsDomainStructMapChanges(t *testing.T) {
	before := snapshotAnalysisResultDomains(&AnalysisResult{
		Call: &CallFacts{CallABIs: map[int]CallABIDescriptor{}},
	})
	after := snapshotAnalysisResultDomains(&AnalysisResult{
		Call: &CallFacts{CallABIs: map[int]CallABIDescriptor{
			7: {NumArgs: 2, NumRets: 1},
		}},
	})

	diffs := diffAnalysisFactDomains(before, after)
	if len(diffs) != 1 {
		t.Fatalf("diffs = %+v, want one domain diff", diffs)
	}
	diff := diffs[0]
	if diff.Domain != "Call.CallABIs" || diff.BeforeCount != 0 || diff.AfterCount != 1 || diff.BeforeHash == diff.AfterHash {
		t.Fatalf("unexpected domain diff: %+v", diff)
	}
}
