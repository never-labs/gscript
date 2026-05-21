package methodjit

// AnalysisFact names a pipeline analysis fact produced and consumed by Tier 2
// optimizer modules.
type AnalysisFact string

const (
	AnalysisFactInlineComplete AnalysisFact = "InlineComplete"

	AnalysisFactFixedShapeTables       AnalysisFact = "FixedShapeTables"
	AnalysisFactFixedShapeEntryGuards  AnalysisFact = "FixedShapeEntryGuards"
	AnalysisFactFieldPolyShapeFacts    AnalysisFact = "FieldPolyShapeFacts"
	AnalysisFactFieldPolyShapeCatalog  AnalysisFact = "FieldPolyShapeCatalog"
	AnalysisFactFixedTableConstructors AnalysisFact = "FixedTableConstructors"

	AnalysisFactSpecDependencyProtos AnalysisFact = "SpecDependencyProtos"

	AnalysisFactCallABIs                 AnalysisFact = "CallABIs"
	AnalysisFactProtocolConstCallFolds   AnalysisFact = "ProtocolConstCallFolds"
	AnalysisFactWholeCallNoResultKernels AnalysisFact = "WholeCallNoResultKernels"
	AnalysisFactWholeCallNoResultBatches AnalysisFact = "WholeCallNoResultBatches"

	AnalysisFactStringConstTables    AnalysisFact = "StringConstTables"
	AnalysisFactStringFormatPatterns AnalysisFact = "StringFormatPatterns"
	AnalysisFactStringSplitSubSpecs  AnalysisFact = "StringSplitSubSpecs"

	AnalysisFactInt48Safe             AnalysisFact = "Int48Safe"
	AnalysisFactIntRanges             AnalysisFact = "IntRanges"
	AnalysisFactIntNonNegative        AnalysisFact = "IntNonNegative"
	AnalysisFactIntModNonZeroDivisor  AnalysisFact = "IntModNonZeroDivisor"
	AnalysisFactIntModNoSignAdjust    AnalysisFact = "IntModNoSignAdjust"
	AnalysisFactTableArrayDataPtrs    AnalysisFact = "TableArrayDataPtrs"
	AnalysisFactShapeFieldTypeElided  AnalysisFact = "ShapeFieldTypeElidedLoads"
	AnalysisFactRecordArrayLoopKernel AnalysisFact = "RecordArrayLoopKernels"
	AnalysisFactRecordArrayLoopCaches AnalysisFact = "RecordArrayLoopCaches"
)

func analysisFacts(facts ...AnalysisFact) []AnalysisFact {
	return facts
}

func fixedShapeTableFacts() []AnalysisFact {
	return analysisFacts(
		AnalysisFactFixedShapeTables,
		AnalysisFactFixedShapeEntryGuards,
		AnalysisFactFieldPolyShapeFacts,
		AnalysisFactFieldPolyShapeCatalog,
		AnalysisFactFixedTableConstructors,
	)
}

func rangeAnalysisFacts() []AnalysisFact {
	return analysisFacts(
		AnalysisFactInt48Safe,
		AnalysisFactIntRanges,
		AnalysisFactIntNonNegative,
		AnalysisFactIntModNonZeroDivisor,
		AnalysisFactIntModNoSignAdjust,
	)
}
