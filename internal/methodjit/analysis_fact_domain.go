package methodjit

import "sort"

// analysis_fact_domain.go defines the six AnalysisResult fact domains and the
// mapping from each modeled AnalysisFact to its owning domain. The read-side
// contract observer (see analysis_read_contract.go) records which domains a
// module accessed through the AnalysisResult accessors and compares that against
// the domains its declared facts (Requires ∪ Provides ∪ Updates ∪ OptionalReads)
// cover.

// factDomain names one of the six AnalysisResult fact domains. The values match
// the domain struct names in lowercase form for stable, human-readable output.
type factDomain string

const (
	factDomainNumeric     factDomain = "numeric"
	factDomainCall        factDomain = "call"
	factDomainSpeculation factDomain = "speculation"
	factDomainTableShape  factDomain = "tableshape"
	factDomainLoopSpec    factDomain = "loopspec"
	factDomainGlobal      factDomain = "global"
)

// analysisFactDomain maps every modeled AnalysisFact to its owning domain. The
// domain is determined by which AnalysisResult domain struct physically holds
// the fact's backing map (see analysis_result.go). Facts not present here (e.g.
// the Function-level String* facts, which never surface through an
// AnalysisResult accessor) are intentionally omitted.
var analysisFactDomain = map[AnalysisFact]factDomain{
	// Numeric domain (NumericFacts): integer ranges and arithmetic safety.
	AnalysisFactInt48Safe:            factDomainNumeric,
	AnalysisFactIntRanges:            factDomainNumeric,
	AnalysisFactProfiledIntRanges:    factDomainNumeric,
	AnalysisFactIntNonNegative:       factDomainNumeric,
	AnalysisFactIntModNonZeroDivisor: factDomainNumeric,
	AnalysisFactIntModNoSignAdjust:   factDomainNumeric,

	// Call domain (CallFacts): call ABIs and call-site specializations.
	AnalysisFactCallABIs:                                     factDomainCall,
	AnalysisFactGuardedConstCallFolds:                        factDomainCall,
	AnalysisFactCallSiteNoResultRuntimeSpecializations:       factDomainCall,
	AnalysisFactCallSiteNoResultRuntimeSpecializationBatches: factDomainCall,

	// Speculation domain (SpeculationFacts): inlined proto dependencies.
	AnalysisFactSpecDependencyProtos: factDomainSpeculation,

	// TableShape domain (TableShapeFacts): table/field shape facts. Note that
	// AnalysisFactInlineComplete is a frontend marker not backed by a domain map;
	// it is omitted. TableArrayDataPtrs and RecordArrayLoopSpecializations are
	// physically stored in LoopSpecializationFacts (see below), so they map to
	// the loopspec domain despite their table-oriented names.
	AnalysisFactFixedShapeTables:       factDomainTableShape,
	AnalysisFactFixedShapeEntryGuards:  factDomainTableShape,
	AnalysisFactFieldPolyShapeFacts:    factDomainTableShape,
	AnalysisFactFieldPolyShapeCatalog:  factDomainTableShape,
	AnalysisFactFixedTableConstructors: factDomainTableShape,
	AnalysisFactShapeFieldTypeElided:   factDomainTableShape,

	// LoopSpecialization domain (LoopSpecializationFacts): table-array bound and
	// record-array loop facts. TableArrayDataPtrs and RecordArrayLoop* maps live
	// in this struct.
	AnalysisFactTableArrayDataPtrs:            factDomainLoopSpec,
	AnalysisFactTableArrayBoundsSafe:          factDomainLoopSpec,
	AnalysisFactLoopTableArrayFacts:           factDomainLoopSpec,
	AnalysisFactRecordArrayLoopSpecialization: factDomainLoopSpec,
	AnalysisFactRecordArrayLoopCaches:         factDomainLoopSpec,

	// Global domain (GlobalFacts): cross-proto seeded inputs. These are seeded by
	// the pipeline/manager outside any module run, not produced by a pass, so no
	// module declares them in Provides. They are modeled here so global reads are
	// named (read-contract) and global writes are not flagged as unmodeled
	// (write-contract).
	AnalysisFactGlobals:                 factDomainGlobal,
	AnalysisFactNumericGlobalValues:     factDomainGlobal,
	AnalysisFactGlobalArrayElementFacts: factDomainGlobal,
}

// factAccessObserver records the set of AnalysisResult fact domains accessed
// through the AnalysisResult domain accessors during a single observed module
// run. A nil observer means observation is inactive (the production hot path).
type factAccessObserver struct {
	accessed map[factDomain]bool
}

// note records that the given domain's accessor was invoked. Safe on a nil
// receiver so callers need only the nil-gate on the AnalysisResult field.
func (o *factAccessObserver) note(d factDomain) {
	if o == nil {
		return
	}
	if o.accessed == nil {
		o.accessed = make(map[factDomain]bool, 6)
	}
	o.accessed[d] = true
}

// domains returns the accessed domains as a sorted slice of strings.
func (o *factAccessObserver) domains() []string {
	if o == nil || len(o.accessed) == 0 {
		return nil
	}
	out := make([]string, 0, len(o.accessed))
	for d := range o.accessed {
		out = append(out, string(d))
	}
	sort.Strings(out)
	return out
}
