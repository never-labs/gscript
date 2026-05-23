package methodjit

// AnalysisFact names a pipeline analysis fact produced and consumed by Tier 2
// optimizer modules.
type AnalysisFact string

// AnalysisFactMetadata documents the stable contract for one analysis fact.
// Producers are modules that first establish the fact; modules that refresh it
// should declare Updates instead. Consumers includes non-module consumers such
// as codegen when a fact is intentionally not required by another pass.
type AnalysisFactMetadata struct {
	Owner       string
	Description string
	Producers   []string
	Consumers   []string
}

const (
	AnalysisFactInlineComplete AnalysisFact = "InlineComplete"

	AnalysisFactFixedShapeTables       AnalysisFact = "FixedShapeTables"
	AnalysisFactFixedShapeEntryGuards  AnalysisFact = "FixedShapeEntryGuards"
	AnalysisFactFieldPolyShapeFacts    AnalysisFact = "FieldPolyShapeFacts"
	AnalysisFactFieldPolyShapeCatalog  AnalysisFact = "FieldPolyShapeCatalog"
	AnalysisFactFixedTableConstructors AnalysisFact = "FixedTableConstructors"

	AnalysisFactSpecDependencyProtos AnalysisFact = "SpecDependencyProtos"

	AnalysisFactCallABIs                                     AnalysisFact = "CallABIs"
	AnalysisFactGuardedConstCallFolds                        AnalysisFact = "GuardedConstCallFolds"
	AnalysisFactCallSiteNoResultRuntimeSpecializations       AnalysisFact = "CallSiteNoResultRuntimeSpecializations"
	AnalysisFactCallSiteNoResultRuntimeSpecializationBatches AnalysisFact = "CallSiteNoResultRuntimeSpecializationBatches"

	AnalysisFactStringConstTables    AnalysisFact = "StringConstTables"
	AnalysisFactStringFormatPatterns AnalysisFact = "StringFormatPatterns"
	AnalysisFactStringSplitSubSpecs  AnalysisFact = "StringSplitSubSpecs"

	AnalysisFactInt48Safe                     AnalysisFact = "Int48Safe"
	AnalysisFactIntRanges                     AnalysisFact = "IntRanges"
	AnalysisFactIntNonNegative                AnalysisFact = "IntNonNegative"
	AnalysisFactIntModNonZeroDivisor          AnalysisFact = "IntModNonZeroDivisor"
	AnalysisFactIntModNoSignAdjust            AnalysisFact = "IntModNoSignAdjust"
	AnalysisFactTableArrayDataPtrs            AnalysisFact = "TableArrayDataPtrs"
	AnalysisFactShapeFieldTypeElided          AnalysisFact = "ShapeFieldTypeElidedLoads"
	AnalysisFactRecordArrayLoopSpecialization AnalysisFact = "RecordArrayLoopSpecializations"
	AnalysisFactRecordArrayLoopCaches         AnalysisFact = "RecordArrayLoopCaches"
)

var analysisFactMetadata = map[AnalysisFact]AnalysisFactMetadata{
	AnalysisFactInlineComplete: {
		Owner:       "frontend",
		Description: "Intrinsic lowering has completed before inlining.",
		Producers:   []string{"Intrinsic"},
		Consumers:   []string{"Inline"},
	},
	AnalysisFactFixedShapeTables: {
		Owner:       "table",
		Description: "Fixed table shapes and compatible table object facts are available.",
		Producers:   []string{"FixedShapeTableFacts (pre-inline)"},
		Consumers: []string{
			"CallABI",
			"LoadElimination",
			"FieldLenFold",
			"StaticTableLenFold",
			"EscapeAnalysis",
			"FixedTableConstructorLowering",
			"TablePrealloc",
			"FieldPolyLenPhi",
			"ShapeFieldTypeGuard",
			"LoopRegionVersioning",
			"ScalarPromotion",
			"TableArrayDataPtrFact",
		},
	},
	AnalysisFactFixedShapeEntryGuards: {
		Owner:       "table",
		Description: "Entry guards proving fixed-shape table arguments are recorded.",
		Producers:   []string{"FixedShapeTableFacts (pre-inline)"},
		Consumers:   []string{"FieldSvalsLower", "codegen"},
	},
	AnalysisFactFieldPolyShapeFacts: {
		Owner:       "table",
		Description: "Polymorphic field shape facts are available for split/fusion passes.",
		Producers:   []string{"FixedShapeTableFacts (pre-inline)"},
		Consumers:   []string{"FieldShapeCallSplitPreInline", "FieldCallPolyLenFusion", "FieldShapeCallSplit (experimental)"},
	},
	AnalysisFactFieldPolyShapeCatalog: {
		Owner:       "table",
		Description: "Catalog of polymorphic field shape observations is available.",
		Producers:   []string{"FixedShapeTableFacts (pre-inline)"},
		Consumers:   []string{"diagnostics", "codegen"},
	},
	AnalysisFactFixedTableConstructors: {
		Owner:       "table",
		Description: "Fixed-shape table constructor facts are available.",
		Producers:   []string{"FixedShapeTableFacts (pre-inline)"},
		Consumers:   []string{"FixedTableConstructorLowering"},
	},
	AnalysisFactSpecDependencyProtos: {
		Owner:       "inline",
		Description: "Inlined proto dependency facts are recorded for speculation checks.",
		Producers:   []string{"Inline"},
		Consumers:   []string{"dependency registry"},
	},
	AnalysisFactCallABIs: {
		Owner:       "call",
		Description: "Call sites have ABI annotations for native lowering and call folds.",
		Producers:   []string{"CallABI"},
		Consumers:   []string{"GuardedConstCallFold", "CallSiteRuntimeSpecializationExit", "GuardFieldCallee", "RecordArrayLoopSpecialization", "TableIntArraySpecialization"},
	},
	AnalysisFactGuardedConstCallFolds: {
		Owner:       "call",
		Description: "Guarded const-call folds have been applied.",
		Producers:   []string{"GuardedConstCallFold"},
		Consumers:   []string{"codegen", "diagnostics"},
	},
	AnalysisFactCallSiteNoResultRuntimeSpecializations: {
		Owner:       "call",
		Description: "Whole-call no-result runtime specialization exits are annotated.",
		Producers:   []string{"CallSiteRuntimeSpecializationExit"},
		Consumers:   []string{"codegen"},
	},
	AnalysisFactCallSiteNoResultRuntimeSpecializationBatches: {
		Owner:       "call",
		Description: "Whole-call no-result runtime specialization batches are annotated.",
		Producers:   []string{"CallSiteRuntimeSpecializationExit"},
		Consumers:   []string{"codegen"},
	},
	AnalysisFactStringConstTables: {
		Owner:       "string",
		Description: "String constant table facts are available for native string lowering.",
		Producers:   []string{"StringFacts"},
		Consumers:   []string{"string native lowering", "codegen"},
	},
	AnalysisFactStringFormatPatterns: {
		Owner:       "string",
		Description: "String format patterns are available for native string lowering.",
		Producers:   []string{"StringFacts"},
		Consumers:   []string{"string native lowering", "codegen"},
	},
	AnalysisFactStringSplitSubSpecs: {
		Owner:       "string",
		Description: "String split sub-specializations are available for native string lowering.",
		Producers:   []string{"StringFacts"},
		Consumers:   []string{"string native lowering", "codegen"},
	},
	AnalysisFactInt48Safe: {
		Owner:       "numeric",
		Description: "Integer values proven safe for int48-specialized lowering are available.",
		Producers:   []string{"RangeAnalysis"},
		Consumers:   []string{"LICM", "LICM (post-MatrixRowPtrFactoring)", "codegen"},
	},
	AnalysisFactIntRanges: {
		Owner:       "numeric",
		Description: "Integer range facts are available.",
		Producers:   []string{"RangeAnalysis"},
		Consumers:   []string{"OverflowBoxing", "ModRangeSimplify", "QuadraticStepStrengthReduction", "IntAlgebraSimplify", "TableArrayStaticBounds", "codegen"},
	},
	AnalysisFactIntNonNegative: {
		Owner:       "numeric",
		Description: "Non-negative integer facts are available.",
		Producers:   []string{"RangeAnalysis"},
		Consumers:   []string{"codegen"},
	},
	AnalysisFactIntModNonZeroDivisor: {
		Owner:       "numeric",
		Description: "Modulo divisors proven non-zero are available.",
		Producers:   []string{"RangeAnalysis"},
		Consumers:   []string{"ModZeroCompare"},
	},
	AnalysisFactIntModNoSignAdjust: {
		Owner:       "numeric",
		Description: "Modulo operations proven not to need sign adjustment are available.",
		Producers:   []string{"RangeAnalysis"},
		Consumers:   []string{"codegen"},
	},
	AnalysisFactTableArrayDataPtrs: {
		Owner:       "table",
		Description: "Table array data pointer facts are available for backend lowering.",
		Producers:   []string{"TableArrayDataPtrFact"},
		Consumers:   []string{"codegen"},
	},
	AnalysisFactShapeFieldTypeElided: {
		Owner:       "table",
		Description: "Shape field type guards have elided redundant loads.",
		Producers:   []string{"ShapeFieldTypeGuard"},
		Consumers:   []string{"codegen", "diagnostics"},
	},
	AnalysisFactRecordArrayLoopSpecialization: {
		Owner:       "table",
		Description: "Record-array loop specializations have been identified.",
		Producers:   []string{"RecordArrayLoopSpecialization"},
		Consumers:   []string{"codegen"},
	},
	AnalysisFactRecordArrayLoopCaches: {
		Owner:       "table",
		Description: "Record-array loop cache facts are available.",
		Producers:   []string{"RecordArrayLoopSpecialization"},
		Consumers:   []string{"codegen"},
	},
}

func lookupAnalysisFactMetadata(fact AnalysisFact) (AnalysisFactMetadata, bool) {
	metadata, ok := analysisFactMetadata[fact]
	return metadata, ok
}

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
