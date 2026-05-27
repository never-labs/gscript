package methodjit

import "testing"

func TestAnalysisFactMetadataComplete(t *testing.T) {
	for _, fact := range allAnalysisFacts {
		metadata, ok := lookupAnalysisFactMetadata(fact)
		if !ok {
			t.Fatalf("%s missing metadata", fact)
		}
		if metadata.Owner == "" {
			t.Fatalf("%s metadata missing owner", fact)
		}
		if metadata.Description == "" {
			t.Fatalf("%s metadata missing description", fact)
		}
		if len(metadata.Producers) == 0 {
			t.Fatalf("%s metadata missing producers", fact)
		}
		if len(metadata.Consumers) == 0 {
			t.Fatalf("%s metadata missing consumers", fact)
		}
	}
}

func TestAnalysisFactDomainMappingsHaveMetadata(t *testing.T) {
	for fact := range analysisFactDomain {
		if _, ok := lookupAnalysisFactMetadata(fact); !ok {
			t.Fatalf("%s has a fact-domain mapping but no metadata", fact)
		}
	}
}

func TestAnalysisFactMetadataMatchesRegistry(t *testing.T) {
	registered := make(map[AnalysisFact]bool, len(allAnalysisFacts))
	for _, fact := range allAnalysisFacts {
		if registered[fact] {
			t.Fatalf("%s appears more than once in allAnalysisFacts", fact)
		}
		registered[fact] = true
	}
	for fact := range analysisFactMetadata {
		if !registered[fact] {
			t.Fatalf("%s has metadata but is missing from allAnalysisFacts", fact)
		}
	}
	if len(registered) != len(analysisFactMetadata) {
		t.Fatalf("allAnalysisFacts count=%d metadata count=%d", len(registered), len(analysisFactMetadata))
	}
}

func TestAnalysisFactMetadataDeclaresDomainBackedStatus(t *testing.T) {
	unbacked := map[AnalysisFact]bool{
		AnalysisFactInlineComplete:       true,
		AnalysisFactStringConstTables:    true,
		AnalysisFactStringFormatPatterns: true,
		AnalysisFactStringSplitSubSpecs:  true,
	}
	for fact := range analysisFactMetadata {
		_, domainBacked := analysisFactDomain[fact]
		if !domainBacked && !unbacked[fact] {
			t.Fatalf("%s has metadata but is neither domain-backed nor explicitly unbacked", fact)
		}
		if domainBacked && unbacked[fact] {
			t.Fatalf("%s is both domain-backed and explicitly unbacked", fact)
		}
	}
}
