package methodjit

import "testing"

func TestSnapshotAnalysisResultDomainsPreservesTopLevelMapSnapshots(t *testing.T) {
	a := NewAnalysisResult()
	a.NumericFacts().RecordInt48Safe(42)

	snapshots := snapshotAnalysisResultDomains(a)
	snapshot, ok := snapshots["Numeric.int48Safe"]
	if !ok {
		t.Fatalf("snapshot missing domain map Numeric.int48Safe")
	}
	if snapshot.Count != 1 || snapshot.Hash == 0 {
		t.Fatalf("Numeric.int48Safe snapshot = %+v, want count 1 with nonzero hash", snapshot)
	}
}

func TestSnapshotAnalysisResultDomainsIncludesDomainStructMaps(t *testing.T) {
	a := NewAnalysisResult()
	a.CallFacts().SetCallABIs(map[int]CallABIDescriptor{
		7: {NumArgs: 2, NumRets: 1},
	})

	snapshots := snapshotAnalysisResultDomains(a)
	snapshot, ok := snapshots["Call.callABIs"]
	if !ok {
		t.Fatalf("snapshot missing domain map Call.callABIs")
	}
	if snapshot.Count != 1 || snapshot.Hash == 0 {
		t.Fatalf("Call.callABIs snapshot = %+v, want count 1 with nonzero hash", snapshot)
	}
}

func TestDiffAnalysisFactDomainsReportsDomainStructMapChanges(t *testing.T) {
	beforeResult := NewAnalysisResult()
	beforeResult.CallFacts().SetCallABIs(map[int]CallABIDescriptor{})
	before := snapshotAnalysisResultDomains(beforeResult)

	afterResult := NewAnalysisResult()
	afterResult.CallFacts().SetCallABIs(map[int]CallABIDescriptor{
		7: {NumArgs: 2, NumRets: 1},
	})
	after := snapshotAnalysisResultDomains(afterResult)

	diffs := diffAnalysisFactDomains(before, after)
	var diff AnalysisFactDomainDiff
	found := false
	for _, d := range diffs {
		if d.Domain == "Call.callABIs" {
			diff = d
			found = true
		}
	}
	if !found {
		t.Fatalf("diffs = %+v, want a Call.callABIs domain diff", diffs)
	}
	if diff.BeforeCount != 0 || diff.AfterCount != 1 || diff.BeforeHash == diff.AfterHash {
		t.Fatalf("unexpected domain diff: %+v", diff)
	}
}
