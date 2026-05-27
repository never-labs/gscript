package methodjit

import "testing"

func TestAnalysisFactMetadataComplete(t *testing.T) {
	for _, fact := range knownAnalysisFactsForTest() {
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

func knownAnalysisFactsForTest() []AnalysisFact {
	return []AnalysisFact{
		AnalysisFactInlineComplete,
		AnalysisFactFixedShapeTables,
		AnalysisFactFixedShapeEntryGuards,
		AnalysisFactFieldPolyShapeFacts,
		AnalysisFactFieldPolyShapeCatalog,
		AnalysisFactFixedTableConstructors,
		AnalysisFactSpecDependencyProtos,
		AnalysisFactCallABIs,
		AnalysisFactGuardedConstCallFolds,
		AnalysisFactCallSiteNoResultRuntimeSpecializations,
		AnalysisFactCallSiteNoResultRuntimeSpecializationBatches,
		AnalysisFactStringConstTables,
		AnalysisFactStringFormatPatterns,
		AnalysisFactStringSplitSubSpecs,
		AnalysisFactInt48Safe,
		AnalysisFactIntRanges,
		AnalysisFactProfiledIntRanges,
		AnalysisFactIntNonNegative,
		AnalysisFactIntModNonZeroDivisor,
		AnalysisFactIntModNoSignAdjust,
		AnalysisFactTableArrayDataPtrs,
		AnalysisFactTableArrayBoundsSafe,
		AnalysisFactLoopTableArrayFacts,
		AnalysisFactShapeFieldTypeElided,
		AnalysisFactRecordArrayLoopSpecialization,
		AnalysisFactRecordArrayLoopCaches,
		AnalysisFactGlobals,
		AnalysisFactNumericGlobalValues,
		AnalysisFactGlobalArrayElementFacts,
	}
}

func TestAnalysisFactDomainMappingsHaveMetadata(t *testing.T) {
	for fact := range analysisFactDomain {
		if _, ok := lookupAnalysisFactMetadata(fact); !ok {
			t.Fatalf("%s has a fact-domain mapping but no metadata", fact)
		}
	}
}
