//go:build darwin && arm64

// emit_compile.go contains the Tier 2 compile pipeline for the Method JIT.
// It takes a Function with register allocation and produces executable ARM64 code.
// Includes the emitContext struct, slot assignment, prologue/epilogue generation,
// and block emission.

package methodjit

import (
	"fmt"

	"github.com/never-labs/leia/internal/jit"
	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/vm"
)

// Suppress unused import warnings.
var _ runtime.Value

var _ *vm.FuncProto

// CompileOptions configures optional, diagnostic-only native code generation.
type CompileOptions struct {
	EnableTier2BlockCounters bool
	TraceNativeCalls         bool
	PrintNativeCallTrace     bool
}

// Compile takes a Function with register allocation and produces executable ARM64 code.
func Compile(fn *Function, alloc *RegAllocation) (*CompiledFunction, error) {
	return CompileWithOptions(fn, alloc, CompileOptions{})
}

// CompileWithOptions takes a Function with register allocation and produces
// executable ARM64 code with optional diagnostic instrumentation.
func CompileWithOptions(fn *Function, alloc *RegAllocation, opts CompileOptions) (*CompiledFunction, error) {
	if err := validateCompileInputs(fn, alloc); err != nil {
		return nil, err
	}

	// Ensure Analysis is initialized for functions constructed outside BuildGraph.
	if fn.Analysis == nil {
		fn.Analysis = NewAnalysisResult()
	}

	// Check if any FPR allocations exist (to skip FPR save/restore).
	hasFPR := false
	for _, pr := range alloc.ValueRegs {
		if pr.IsFloat {
			hasFPR = true
			break
		}
	}

	li := computeLoopInfo(fn)
	crossBlockLive := computeCrossBlockLive(fn)
	blockLiveIn, blockLiveOut := computeBlockLiveness(fn)
	instrLiveAfter := computeInstrLiveAfter(fn, blockLiveOut)
	useCounts := computeUseCounts(fn)
	valueDefs := computeValueDefs(fn)
	rawIntBlockCarry := enableSinglePredRawIntCarry(fn)
	rawIntCarryNoStore := map[int]bool(nil)
	if rawIntBlockCarry {
		rawIntCarryNoStore = computeSinglePredRawIntStoreElision(fn, alloc, blockLiveIn)
	}
	rawFloatCarryNoStore := computeSinglePredRawFloatStoreElision(fn, alloc, blockLiveIn)
	defs := make(map[int]*Instr)
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if !instr.Op.IsTerminator() {
				defs[instr.ID] = instr
			}
		}
	}
	var headerRegs map[int]map[int]loopRegEntry
	var headerFPRegs map[int]map[int]loopFPRegEntry
	var safeHdrRegs map[int]map[int]loopRegEntry
	var safeHdrFPRegs map[int]map[int]loopFPRegEntry
	var loopInvariantGPRs map[int]map[int]loopRegEntry
	var loopInvariantFPRs map[int]map[int]loopFPRegEntry
	var phiOnlyArgs loopPhiArgSet
	var fpPhiOnlyArgs loopPhiArgSet
	exitBoxPhis := make(map[int]bool)
	exitBoxFPPhis := make(map[int]bool)
	exitStorePhis := make(map[int]bool)
	// Identify single-use comparisons that can be fused with their
	// immediately-following Branch. Several loop analyses need this same fact;
	// compute it once for the compile instead of rebuilding use counts.
	fusedCmps := computeFusedComparisons(fn)
	if li.hasLoops() {
		headerRegs = li.computeHeaderExitRegs(fn, alloc, fusedCmps)
		headerFPRegs = li.computeHeaderExitFPRegs(fn, alloc)
		// Compute safe header regs: only registers NOT clobbered by any
		// non-header block in the loop body. These are used for both
		// block activation and loopPhiOnlyArgs checking.
		safeHdrRegs = computeSafeHeaderRegs(fn, li, alloc, headerRegs, fusedCmps)
		safeHdrFPRegs = computeSafeHeaderFPRegs(fn, li, alloc, headerFPRegs)
		loopInvariantGPRs = computeSafeLoopInvariantGPRs(fn, li, alloc, fusedCmps)
		loopInvariantFPRs = computeSafeLoopInvariantFPRs(fn, li, alloc)
		phiOnlyArgs = computeLoopPhiArgs(fn, li, alloc, safeHdrRegs)
		fpPhiOnlyArgs = computeLoopFPPhiArgs(fn, li, alloc, safeHdrFPRegs)

		// Identify loop header phis that need exit-time boxing:
		// cross-block live AND register survives through the ENTIRE loop body
		// (not just the header). If any non-header block in the loop has an
		// instruction allocated to the same GPR, the phi's register will be
		// clobbered, so we must write-through on every iteration.
		for headerID, phiIDs := range li.loopPhis {
			hdrRegs := headerRegs[headerID]
			bodyBlocks := li.headerBlocks[headerID]
			for _, phiID := range phiIDs {
				if !crossBlockLive[phiID] {
					continue
				}
				pr, ok := alloc.ValueRegs[phiID]
				if !ok || pr.IsFloat {
					continue
				}
				// Check if this phi's register still holds this phi at
				// end of its own header.
				entry, inRegs := hdrRegs[pr.Reg]
				if !inRegs || entry.ValueID != phiID || !entry.IsRawInt {
					continue
				}
				// Check that no non-header block in the loop body clobbers
				// this register. If clobbered, the phi value can't survive
				// in-register and must be written to memory.
				//
				// A "clobber" is any instruction whose allocated register
				// equals this phi's register. Nested loop header phis
				// count: their phi moves write the register at inner-loop
				// entry, overwriting the outer header's phi value.
				clobbered := false
				for _, block := range fn.Blocks {
					if block.ID == headerID || !bodyBlocks[block.ID] {
						continue
					}
					for _, instr := range block.Instrs {
						if instr.Op.IsTerminator() {
							continue
						}
						instrPR, ok := alloc.ValueRegs[instr.ID]
						if !ok || instrPR.IsFloat || instrPR.Reg != pr.Reg {
							continue
						}
						clobbered = true
						break
					}
					if clobbered {
						break
					}
				}
				if !clobbered {
					exitBoxPhis[phiID] = true
				}
			}
		}

		for headerID, phiIDs := range li.loopPhis {
			hdrRegs := headerRegs[headerID]
			bodyBlocks := li.headerBlocks[headerID]
			if loopBodyHasDirectDeopt(fn, bodyBlocks) {
				continue
			}
			for _, phiID := range phiIDs {
				if !crossBlockLive[phiID] {
					continue
				}
				phi := defs[phiID]
				if phi == nil || phi.Type == TypeInt {
					continue
				}
				pr, ok := alloc.ValueRegs[phiID]
				if !ok || pr.IsFloat {
					continue
				}
				entry, inRegs := hdrRegs[pr.Reg]
				if !inRegs || entry.ValueID != phiID || entry.IsRawInt {
					continue
				}
				clobbered := false
				for _, block := range fn.Blocks {
					if block.ID == headerID || !bodyBlocks[block.ID] {
						continue
					}
					for _, instr := range block.Instrs {
						if instr.Op.IsTerminator() {
							continue
						}
						instrPR, ok := alloc.ValueRegs[instr.ID]
						if ok && !instrPR.IsFloat && instrPR.Reg == pr.Reg {
							clobbered = true
							break
						}
					}
					if clobbered {
						break
					}
				}
				if !clobbered {
					exitStorePhis[phiID] = true
				}
			}
		}

		for headerID, phiIDs := range li.loopPhis {
			hdrFPRegs := headerFPRegs[headerID]
			bodyBlocks := li.headerBlocks[headerID]
			for _, phiID := range phiIDs {
				if !crossBlockLive[phiID] {
					continue
				}
				pr, ok := alloc.ValueRegs[phiID]
				if !ok || !pr.IsFloat {
					continue
				}
				entry, inRegs := hdrFPRegs[pr.Reg]
				if !inRegs || entry.ValueID != phiID {
					continue
				}
				clobbered := false
				for _, block := range fn.Blocks {
					if block.ID == headerID || !bodyBlocks[block.ID] {
						continue
					}
					for _, instr := range block.Instrs {
						if instr.Op.IsTerminator() {
							continue
						}
						instrPR, ok := alloc.ValueRegs[instr.ID]
						if !ok || !instrPR.IsFloat || instrPR.Reg != pr.Reg {
							continue
						}
						clobbered = true
						break
					}
					if clobbered {
						break
					}
				}
				if !clobbered {
					exitBoxFPPhis[phiID] = true
				}
			}
		}
	}

	// Build constant int/bool maps for immediate optimization, and IR type map for
	// resolveRawFloat to detect int-typed values that need SCVTF conversion.
	constInts := make(map[int]int64)
	constBools := make(map[int]int64)
	irTypes := make(map[int]Type)
	irDefs := make(map[int]*Instr)
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op == OpConstInt {
				constInts[instr.ID] = instr.Aux
			}
			if instr.Op == OpConstBool {
				constBools[instr.ID] = instr.Aux
			}
			irTypes[instr.ID] = instr.Type
			irDefs[instr.ID] = instr
		}
	}

	nativeCallReplaySafe := tier2NativeCallReplaySafe(fn)
	nativeCallCalleeResumeSafe := tier2NativeCallCalleeResumeSafe(fn)
	rawIntSelfABI := AnalyzeRawIntSelfABI(fn.Proto)
	typedSelfABI := AnalyzeTypedSelfABI(fn.Proto)
	globalFacts := functionGlobalFacts(fn)
	tableShapeFacts := functionTableShapeFacts(fn)
	typedPeerABI := AnalyzeTypedPeerABIWithFactsAndGlobals(fn.Proto, nil, nil, globalFacts.NumericGlobalValuesMap(), globalFacts.GlobalArrayElementFactsMap())
	typedEntryABI := typedSelfABI
	if !typedEntryABI.Eligible && typedPeerABI.Eligible {
		typedEntryABI = typedPeerABI
	}

	ensureTier2FieldCachesForFunction(fn)
	newTableCaches := newTableCacheSlotsForFunction(fn)
	if !exitResumeCheckEnabled() {
		prewarmNewTableCachesForFunction(fn, newTableCaches)
	}

	ec := &emitContext{
		fn:                         fn,
		alloc:                      alloc,
		asm:                        jit.NewAssembler(),
		slotMap:                    make(map[int]int),
		nextSlot:                   fn.NumRegs,
		activeRegs:                 make(map[int]bool),
		valueReprs:                 make(map[int]valueRepr),
		rawIntRegs:                 make(map[int]bool),
		rawTablePtrRegs:            make(map[int]bool),
		activeFPRegs:               make(map[int]bool),
		shapeVerified:              make(map[int]uint32),
		tableVerified:              make(map[int]bool),
		kindVerified:               make(map[int]uint16),
		keysDirtyWritten:           make(map[int]bool),
		stringLookupCleanGuarded:   make(map[int]bool),
		localNewTablesNoMetatable:  functionHasNoTableMetatableMutationSurface(fn),
		tableArrayBoundedKeys:      make(map[tableArrayBoundKey]bool),
		dmVerified:                 make(map[int]bool),
		blockOutShapes:             make(map[int]map[int]uint32),
		blockOutTables:             make(map[int]map[int]bool),
		blockOutKinds:              make(map[int]map[int]uint16),
		blockOutKeysDirty:          make(map[int]map[int]bool),
		blockOutRawIntRegs:         make(map[int]map[int]loopRegEntry),
		blockOutRawFloatRegs:       make(map[int]map[int]loopFPRegEntry),
		blockLiveIn:                blockLiveIn,
		blockLiveOut:               blockLiveOut,
		instrLiveAfter:             instrLiveAfter,
		useCounts:                  useCounts,
		valueDefs:                  valueDefs,
		rawIntBlockCarry:           rawIntBlockCarry,
		rawIntCarryNoStore:         rawIntCarryNoStore,
		rawFloatCarryNoStore:       rawFloatCarryNoStore,
		crossBlockLive:             crossBlockLive,
		globalCacheConsts:          make([]int, 0),
		useFPR:                     hasFPR,
		loop:                       li,
		loopHeaderRegs:             headerRegs,
		loopHeaderFPRegs:           headerFPRegs,
		safeHeaderRegs:             safeHdrRegs,
		safeHeaderFPRegs:           safeHdrFPRegs,
		loopInvariantGPRs:          loopInvariantGPRs,
		loopInvariantFPRs:          loopInvariantFPRs,
		loopPhiOnlyArgs:            phiOnlyArgs,
		loopFPPhiOnlyArgs:          fpPhiOnlyArgs,
		loopExitBoxPhis:            exitBoxPhis,
		loopExitBoxFPPhis:          exitBoxFPPhis,
		loopExitStorePhis:          exitStorePhis,
		constInts:                  constInts,
		constBools:                 constBools,
		irTypes:                    irTypes,
		irDefs:                     irDefs,
		scratchFPRCache:            make(map[int]jit.FReg),
		fusedCmps:                  fusedCmps,
		tailCallInstrs:             computeTailCalls(fn),
		newTableCaches:             newTableCaches,
		exitResumeLive:             make(exitResumeLiveMetadata),
		fixedTableArgSlots:         make(map[int][]int),
		instrCodeRanges:            make([]InstrCodeRange, 0, fn.nextID),
		nativeCallReplaySafe:       nativeCallReplaySafe,
		nativeCallCalleeResumeSafe: nativeCallCalleeResumeSafe,
		rawIntSelfABI:              rawIntSelfABI,
		typedSelfABI:               typedEntryABI,
		entryShapeGuards:           tableShapeFacts.FixedShapeEntryGuardMap(),
		traceNativeCalls:           opts.TraceNativeCalls,
		printNativeCallTrace:       opts.PrintNativeCallTrace,
	}
	if opts.EnableTier2BlockCounters {
		ec.initTier2BlockCounters()
		ec.initTier2CallCounters()
	}
	if exitResumeCheckEnabled() {
		ec.exitResumeCheck = newExitResumeCheckMetadata()
	}
	// R124/R126: numeric entry is emitted as pass-2 body inside this
	// Compile when the proto qualifies. numericParamCount tells the
	// post-epilogue dispatcher whether to run pass 2.
	if rawIntSelfABI.Eligible {
		ec.numericParamCount = rawIntSelfABI.NumParams
		ec.numericParamSlots = make(map[int]bool, rawIntSelfABI.NumParams)
		for i := 0; i < rawIntSelfABI.NumParams; i++ {
			ec.numericParamSlots[i] = true
		}
	}

	// Assign home slots for all SSA values.
	ec.assignSlots()

	shiftAddVersion, hasShiftAddVersion := detectShiftAddOverflowVersion(fn)
	ec.skipStandardDirectEntry = hasShiftAddVersion

	// Emit prologue.
	ec.emitPrologue()
	ec.emitGuardedConstCallEntryGuards()

	if hasShiftAddVersion {
		ec.emitShiftAddOverflowVersion(shiftAddVersion)
	} else {
		// Emit each basic block.
		for _, block := range fn.Blocks {
			ec.emitBlock(block)
		}
	}

	// Emit epilogue.
	ec.emitEpilogue()
	if hasShiftAddVersion {
		ec.emitShiftAddOverflowVersionDirect(shiftAddVersion)
	}

	// R129: emit pass-2 (numeric variant) body BEFORE deferredResumes so
	// pass-2's deopts/call-exits append to the same deferredResumes
	// list. emitDeferredResumes then emits both passes' resume entries
	// with properly-disambiguated labels (numericPass flag on each).
	if !hasShiftAddVersion {
		ec.emitNumericBody()
	}

	// Emit deferred resume entry points (after epilogue so they're separate
	// function entry points with their own prologue).
	ec.emitDeferredResumes()

	// Finalize: resolve labels.
	code, err := ec.asm.Finalize()
	if err != nil {
		return nil, fmt.Errorf("methodjit: finalize error: %w", err)
	}

	// Allocate executable memory and write code.
	cb, err := jit.AllocExec(len(code) + 1024) // extra space for safety
	if err != nil {
		return nil, fmt.Errorf("methodjit: alloc exec error: %w", err)
	}
	if err := cb.WriteCode(code); err != nil {
		cb.Free()
		return nil, fmt.Errorf("methodjit: write code error: %w", err)
	}

	// Resolve pass-specific resume addresses for exit-resume points.
	resumeAddrs := make(map[int]int)
	numericResumeAddrs := make(map[int]int)
	for _, dr := range ec.deferredResumes {
		label := callExitResumeLabelForPass(dr.instrID, dr.numericPass)
		off := ec.asm.LabelOffset(label)
		if off < 0 {
			continue
		}
		if dr.numericPass {
			numericResumeAddrs[dr.instrID] = off
		} else {
			resumeAddrs[dr.instrID] = off
		}
	}
	exitSites := buildExitSiteMeta(fn)
	continuations := buildTier2Continuations(exitSites, ec.deferredResumes, ec.exitResumeLive, fn.NumRegs, ec.asm.LabelOffset)

	// Resolve direct entry offset for BLR callers.
	directEntryOff := ec.asm.LabelOffset("t2_direct_entry")
	leafEntryOff := ec.asm.LabelOffset("t2_leaf_entry")
	numericEntryOff := 0
	if ec.numericParamCount > 0 {
		label := fmt.Sprintf("t2_numeric_self_entry_%d", ec.numericParamCount)
		if off := ec.asm.LabelOffset(label); off >= 0 {
			numericEntryOff = off
		}
	}
	typedEntryOff := 0
	typedClobberEntryOff := 0
	if typedEntryABI.Eligible {
		if off := ec.asm.LabelOffset("t2_typed_self_entry"); off >= 0 {
			typedEntryOff = off
		}
		if typedPeerClobberABIEnabled(typedEntryABI) {
			if off := ec.asm.LabelOffset("t2_typed_peer_clobber_entry"); off >= 0 {
				typedClobberEntryOff = off
			}
		}
	}
	typedPeerFramePlan := AnalyzeTypedPeerFramePlan(fn, alloc, typedPeerABI)

	// Allocate per-GetGlobal value cache if any GetGlobal instructions exist.
	var globalCache []uint64
	if ec.nextGlobalCacheIndex > 0 {
		globalCache = make([]uint64, ec.nextGlobalCacheIndex)
	}
	nativeSetGlobals := collectNativeSetGlobals(fn)
	globalGuardConsts := collectGlobalGuardConsts(fn)

	// R108/R151/Ractors: allocate per-OpCall polymorphic IC cache.
	var callCache []uint64
	if ec.nextCallCacheIndex > 0 {
		callCache = make([]uint64, tier2CallCacheStrideWords*ec.nextCallCacheIndex)
	}

	return &CompiledFunction{
		Code:                     cb,
		Proto:                    fn.Proto,
		NumSpills:                alloc.NumSpillSlots,
		numRegs:                  ec.nextSlot,
		ResumeAddrs:              resumeAddrs,
		NumericResumeAddrs:       numericResumeAddrs,
		DirectEntryOffset:        directEntryOff,
		LeafEntryOffset:          leafEntryOff,
		DirectEntrySafe:          nativeCallReplaySafe,
		Tier2DirectEntrySafe:     nativeCallCalleeResumeSafe,
		NumericParamCount:        rawIntSelfABI.NumParams,
		RawIntSelfABI:            rawIntSelfABI,
		NumericEntryOffset:       numericEntryOff,
		TypedSelfABI:             typedSelfABI,
		TypedPeerABI:             typedPeerABI,
		TypedEntryOffset:         typedEntryOff,
		TypedClobberEntryOffset:  typedClobberEntryOff,
		TypedPeerFramePlan:       typedPeerFramePlan,
		GlobalCache:              globalCache,
		GlobalCacheConsts:        ec.globalCacheConsts,
		GlobalGuardConsts:        globalGuardConsts,
		NativeSetGlobals:         nativeSetGlobals,
		CallCache:                callCache,
		CallCachePCs:             ec.callCachePCs,
		NewTableCaches:           ec.newTableCaches,
		FixedTableArgSlots:       ec.fixedTableArgSlots,
		FixedRecordNewTableSites: tableShapeFacts.FixedRecordNewTableSiteMap(),
		StringConstTables:        fn.StringConstTables,
		StringFormatPatterns:     fn.StringFormatPatterns,
		StringSplitSubSpecs:      fn.StringSplitSubSpecs,
		CallSiteNoResultRuntimeSpecializationBatches: functionCallFacts(fn).CallSiteNoResultRuntimeSpecializationBatchMap(),
		RecordArrayLoopCaches:                        fn.RecordArrayLoopCaches,
		InstrCodeRanges:                              ec.instrCodeRanges,
		ExitSites:                                    exitSites,
		Continuations:                                continuations,
		ExitResumeCheck:                              ec.exitResumeCheck,
		Tier2BlockCounters:                           ec.tier2BlockCounters,
		Tier2BlockCounterMeta:                        ec.tier2BlockCounterMeta,
		Tier2CallCounters:                            ec.tier2CallCounters,
		Tier2CallCounterMeta:                         ec.tier2CallCounterMeta,
	}, nil
}

func validateCompileInputs(fn *Function, alloc *RegAllocation) error {
	if fn == nil {
		return fmt.Errorf("methodjit: compile nil function")
	}
	if alloc == nil {
		return fmt.Errorf("methodjit: compile nil register allocation")
	}
	if fn.Entry == nil {
		return fmt.Errorf("methodjit: compile function has nil entry block")
	}
	if len(fn.Blocks) == 0 {
		return fmt.Errorf("methodjit: compile function has no blocks")
	}
	entryFound := false
	for _, block := range fn.Blocks {
		if block == fn.Entry {
			entryFound = true
			break
		}
	}
	if !entryFound {
		return fmt.Errorf("methodjit: compile entry block B%d is missing from block list", fn.Entry.ID)
	}
	return nil
}

func collectNativeSetGlobals(fn *Function) map[int]bool {
	out := make(map[int]bool)
	if fn == nil {
		return out
	}
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpSetGlobal || instr.Aux < 0 {
				continue
			}
			out[int(instr.Aux)] = true
		}
	}
	return out
}

func collectGlobalGuardConsts(fn *Function) []int {
	seen := make(map[int]bool)
	var out []int
	if fn == nil {
		return out
	}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpGuardGlobalConst {
				continue
			}
			constIdx := int(instr.Aux)
			if seen[constIdx] {
				continue
			}
			seen[constIdx] = true
			out = append(out, constIdx)
		}
	}
	return out
}

func ensureTier2FieldCachesForFunction(fn *Function) {
	if fn == nil || fn.Proto == nil {
		return
	}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr == nil || instr.SourcePC < 0 {
				continue
			}
			if opNeedsTier2FieldCache(instr.Op) {
				ensureFieldCache(fn.Proto)
				return
			}
		}
	}
}

func fnSupportsIndexedGlobalProtocol(fn *Function) bool {
	return fn != nil && fn.Proto != nil
}

func protoSupportsIndexedGlobalProtocol(proto *vm.FuncProto) bool {
	return proto != nil
}

func fnSupportsNativeSetGlobalProtocol(fn *Function) bool {
	if fn == nil || !protoSupportsNativeSetGlobalProtocol(fn.Proto) {
		return false
	}
	// Result-producing op-exits can materialize values that are immediately
	// published to globals and then consumed by VM calls before the function
	// returns. Keep those global writes on the VM path until the op has a
	// fully native lowering.
	return !fnHasResultProducingOpExit(fn)
}

func protoSupportsNativeSetGlobalProtocol(proto *vm.FuncProto) bool {
	return proto != nil && proto.Name == "<main>"
}

func fnHasResultProducingOpExit(fn *Function) bool {
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr == nil || instructionHasNoSSAResult(instr) {
				continue
			}
			switch instr.Op {
			case OpMatrixDense, OpVectorGather, OpVectorCompare:
				return true
			}
		}
	}
	return false
}

func loopBodyHasDirectDeopt(fn *Function, bodyBlocks map[int]bool) bool {
	for _, block := range fn.Blocks {
		if !bodyBlocks[block.ID] {
			continue
		}
		for _, instr := range block.Instrs {
			if instrMayDirectDeoptWithoutFullFlush(instr) {
				return true
			}
		}
	}
	return false
}

func instrMayDirectDeoptWithoutFullFlush(instr *Instr) bool {
	if instr == nil {
		return false
	}
	if opMayDirectDeoptWithoutFullFlush(instr.Op) {
		return true
	}
	return instr.Op == OpGetField && instr.Type == TypeFloat
}

// emitContext holds transient state during code generation.
type emitContext struct {
	fn           *Function
	alloc        *RegAllocation
	asm          *jit.Assembler
	slotMap      map[int]int // SSA value ID -> home slot index in VM register file
	nextSlot     int         // next available temp slot
	labelCounter int         // counter for generating unique labels

	// activeRegs tracks which value IDs have their register allocation active
	// in the current block. Values from other blocks must be loaded from memory.
	// Reset at the start of each block.
	activeRegs map[int]bool

	// crossBlockLive is the set of value IDs that are used in blocks other than
	// where they're defined. These values need write-through to memory.
	// Values only used within their defining block skip the memory write.
	crossBlockLive map[int]bool

	// valueReprs is the explicit representation lattice for active allocated
	// registers. Boxed is the default absence value. rawIntRegs and
	// rawTablePtrRegs remain compatibility mirrors rebuilt from valueReprs.
	valueReprs map[int]valueRepr

	// rawIntRegs tracks which value IDs have RAW int64 (not NaN-boxed) content
	// in their allocated register. Compatibility mirror for valueReprs.
	rawIntRegs map[int]bool

	// rawTablePtrRegs tracks value IDs whose allocated GPR holds a raw
	// *runtime.Table pointer. Home slots always hold a boxed table Value so
	// exit-resume and GC-visible VM state never see raw pointers.
	// Compatibility mirror for valueReprs.
	rawTablePtrRegs map[int]bool

	// shapeVerified tracks table SSA value IDs whose shape has been verified
	// in the current block. Maps table value ID -> verified shapeID.
	// Reset at block boundaries and after calls.
	shapeVerified map[int]uint32

	// tableVerified tracks table SSA value IDs whose table identity
	// (type check, nil check, metatable check) has been verified in the
	// current block. Maps table value ID -> true.
	// Reset at block boundaries and after calls (same as shapeVerified).
	tableVerified map[int]bool

	// keysDirtyWritten tracks table SSA value IDs whose keysDirty byte
	// has already been written to 1 in the current block. Subsequent
	// SetTables on the same table elide the MOVimm16+STRB (2 insns).
	// The flag is idempotent (always set to 1), so consecutive writes
	// produce the same final state. Reset at block boundaries and after
	// calls (same as tableVerified).
	keysDirtyWritten map[int]bool

	// stringLookupCleanGuarded tracks table SSA value IDs whose string lookup
	// cache/version pointer has been guarded as nil in the current block.
	// Shape-stable SetField stores can then skip per-store version bumps until
	// a block boundary or call-like barrier resets the state.
	stringLookupCleanGuarded map[int]bool

	// localNewTablesNoMetatable is true when this function has no call/op-exit
	// surface that can install a metatable on tables allocated by OpNewTable.
	localNewTablesNoMetatable bool

	// kindVerified tracks table SSA value IDs whose ArrayKind has been
	// guard-checked in the current block. Maps table value ID -> the
	// AKKind constant (jit.AKMixed/AKInt/AKFloat/AKBool) last verified.
	// When an emit path is about to emit a knownKind kind guard and the
	// map entry already equals that kind, the guard (LDRB+CMP+BCond+B)
	// is skipped — just emit the direct B to the matching label.
	// Reset at block boundaries and after calls (same as tableVerified).
	// Invalidated at the END of each GetTable/SetTable emission because
	// the deopt path can enter runtime code that may demote the array
	// kind (same conservative pattern as tableVerified).
	kindVerified map[int]uint16

	// dmVerified tracks table SSA value IDs that have been confirmed as
	// DenseMatrix outers (dmStride > 0) in the current block. Lets
	// subsequent matrix.getf/setf calls on the same m skip the stride
	// guard LDR+CBZ. Reset at block boundaries and after calls.
	// Populated by emitMatrixGetF/emitMatrixSetF (R44).
	dmVerified map[int]bool

	// fieldSvalsCache tracks X1 when it is known to hold Table.svals data for
	// a verified (table SSA value, shape) pair. It is scoped to consecutive
	// field ops in one emitted block because X1 is scratch everywhere else.
	fieldSvalsCacheValid   bool
	fieldSvalsCacheTableID int
	fieldSvalsCacheShapeID uint32

	// blockOutShapes saves the shapeVerified state at the END of each emitted block.
	// Used to seed single-predecessor blocks with their predecessor's verified shapes.
	blockOutShapes map[int]map[int]uint32

	// blockOutTables saves the tableVerified state at the END of each emitted block.
	blockOutTables map[int]map[int]bool

	// blockOutKinds saves the kindVerified state at the END of each emitted
	// block. Used to seed single-predecessor blocks with their predecessor's
	// verified kinds (mirrors blockOutTables).
	blockOutKinds map[int]map[int]uint16

	// blockOutKeysDirty saves the keysDirtyWritten state at end of block.
	blockOutKeysDirty map[int]map[int]bool

	// tableArrayBoundedKeys tracks instruction-local (table, key) pairs whose
	// immediately preceding TableArrayLoad has a live native-success flag in
	// X17. The flag is set on the native load success path and cleared on the
	// exit-resume path, so a following SetTable can branch around its redundant
	// bounds check without assuming the load succeeded after resume.
	tableArrayBoundedKeys map[tableArrayBoundKey]bool

	// blockOutRawIntRegs saves the raw-int GPR state at end of block, keyed
	// by block ID then physical register. Single-predecessor successors can
	// activate these values when liveness says they are live-in.
	blockOutRawIntRegs map[int]map[int]loopRegEntry

	// blockOutRawFloatRegs saves raw-float FPR state at end of block for
	// predecessor-edge activation. It does not affect register allocation.
	blockOutRawFloatRegs map[int]map[int]loopFPRegEntry

	// blockLiveIn is block-level SSA liveness used to bound raw-int carry.
	blockLiveIn map[int]map[int]bool

	// blockLiveOut and instrLiveAfter bound call spills and active-state
	// lifetime for values carried across block boundaries.
	blockLiveOut   map[int]map[int]bool
	instrLiveAfter map[int]map[int]bool
	useCounts      map[int]int
	valueDefs      map[int]*Instr

	rawIntBlockCarry bool

	// rawIntCarryNoStore marks raw-int values whose cross-block uses are
	// covered by immediate single-predecessor carry. Their boxed SSA homes are
	// materialized only on deopt/fallback while live.
	rawIntCarryNoStore map[int]bool

	// rawFloatCarryNoStore marks raw-float values whose cross-block uses are
	// covered by predecessor-edge FPR carry.
	rawFloatCarryNoStore map[int]bool

	// activeFPRegs tracks which value IDs have their FPR allocation active
	// in the current block. Mirrors activeRegs for FPR-allocated values.
	// Reset at the start of each block.
	activeFPRegs map[int]bool

	// useFPR is true if any values are allocated to FPR registers.
	// When false, FPR save/restore in prologue/epilogue is skipped.
	useFPR bool

	// callExitIDs tracks the instruction IDs of call-exit points.
	// After finalization, these are used to resolve resume label addresses.
	callExitIDs []int

	// deferredResumes tracks resume entry points to emit after the epilogue.
	deferredResumes []deferredResume

	// exitResumeLive records lightweight continuation live-state metadata for
	// diagnostics and future safe mid-run version switching. It is independent
	// from the opt-in shadow verifier below.
	exitResumeLive exitResumeLiveMetadata

	// loop tracks loop structure for raw-int loop optimization.
	// When non-nil and a block is inside a loop, emitPhiMoves to loop
	// headers transfers raw ints, and loop header phis are marked rawInt.
	loop *loopInfo

	// loopHeaderRegs is the per-header register state at the end of each loop
	// header. Maps headerBlockID -> (register number -> entry). Non-header
	// loop blocks look up their innermost header to activate the right registers.
	loopHeaderRegs map[int]map[int]loopRegEntry

	// loopHeaderFPRegs is the per-header FPR register state at the end of
	// each loop header. Maps headerBlockID -> (FPR number -> entry).
	loopHeaderFPRegs map[int]map[int]loopFPRegEntry

	// safeHeaderRegs are the subset of loopHeaderRegs whose registers are
	// NOT clobbered by any non-header block in the loop body. Only these
	// values can safely be activated in non-header blocks.
	safeHeaderRegs   map[int]map[int]loopRegEntry
	safeHeaderFPRegs map[int]map[int]loopFPRegEntry

	// loopInvariantGPRs are selected loop-invariant GPR values (currently
	// typed-array len/data facts) whose registers are pinned by regalloc and
	// can be activated in every block of the owning loop.
	loopInvariantGPRs map[int]map[int]loopRegEntry
	// loopInvariantFPRs are selected loop-invariant float values whose
	// registers are pinned by regalloc and can be activated in every block of
	// the owning loop.
	loopInvariantFPRs map[int]map[int]loopFPRegEntry

	// loopPhiOnlyArgs is the set of value IDs that are ONLY used as phi args
	// to loop header phis. storeRawInt skips write-through for these values
	// since emitPhiMoveRawInt reads from the register directly.
	loopPhiOnlyArgs loopPhiArgSet
	// loopFPPhiOnlyArgs is the FPR equivalent for raw-float values.
	loopFPPhiOnlyArgs loopPhiArgSet

	// loopExitBoxPhis is the set of phi value IDs that need boxing at loop
	// exit. These are loop header phis that are cross-block live (used
	// outside the loop) but whose write-through is deferred to exit time.
	loopExitBoxPhis map[int]bool
	// loopExitBoxFPPhis is the FPR equivalent for raw-float header phis.
	loopExitBoxFPPhis map[int]bool
	// loopExitStorePhis is the NaN-boxed GPR equivalent: header phis whose
	// register already holds the boxed runtime Value can defer memory write-
	// through until leaving the loop.
	loopExitStorePhis map[int]bool

	// currentBlockID is the ID of the block currently being emitted.
	currentBlockID int

	// traceNativeCalls enables compile-time native-call diagnostics in remarks.
	// printNativeCallTrace additionally mirrors those diagnostics to stderr.
	traceNativeCalls     bool
	printNativeCallTrace bool

	// constInts maps value ID -> int64 for ConstInt instructions.
	// Used by emitRawIntBinOp to emit ADDimm/SUBimm for small constants.
	constInts map[int]int64

	// constBools maps value ID -> 0 (false) or 1 (true) for ConstBool instructions.
	// Used by emitSetTableNative to bypass runtime tag checks for constant bools.
	constBools map[int]int64

	// irTypes maps value ID -> IR Type from the defining instruction.
	// Used by resolveRawFloat to detect NaN-boxed ints that need SCVTF
	// conversion instead of FMOVtoFP.
	irTypes map[int]Type
	irDefs  map[int]*Instr

	// nextGlobalCacheIndex is the next available cache slot index for
	// OpGetGlobal native cache. Each GetGlobal instruction gets a unique
	// index (0, 1, 2, ...) assigned at emission time.
	nextGlobalCacheIndex int
	globalCacheConsts    []int

	// nextCallCacheIndex (R108) assigns a unique IC slot to each OpCall
	// in the compiled function. 4 uint64 per slot (closure value,
	// direct-entry addr, proto ptr, direct-entry version). Incremented in
	// emitCallNative.
	nextCallCacheIndex int
	callCachePCs       []int

	// fixedTableArgSlots records VM home slots for N-field fixed constructors
	// whose exit fallback gathers an arbitrary number of constructor values.
	fixedTableArgSlots map[int][]int

	// scratchFPRCache maps value ID -> scratch FPR (D0-D3) currently holding
	// that value's raw float. It is scoped to one emitted block and may survive
	// across adjacent pure raw-float instructions. Any instruction that can
	// clobber D0-D3 clears it, and raw-float emitters invalidate a scratch FPR
	// before writing a result to it.
	scratchFPRCache map[int]jit.FReg

	// fusedCmps is the set of comparison instruction IDs that will be fused
	// with their immediately-following Branch. These comparisons emit only
	// CMP/FCMP (no CSET+ORR bool materialization).
	fusedCmps map[int]bool

	// tailCallInstrs (R107) is the set of OpCall instruction IDs that are
	// in tail position: their result is consumed by the immediately-following
	// OpReturn in the same block. Populated by computeTailCalls at
	// emitContext construction. The tail-call emit does a BR to the
	// callee's direct entry on the fast path; the following OpReturn is
	// emitted as dead code (fast-path never falls through) but remains
	// live on the slow-path fallback (emitCallExitFallback produces a
	// normal return value that the Return then handles).
	tailCallInstrs map[int]bool

	// newTableCaches is owned by the eventual CompiledFunction but allocated
	// before emission so native NewTable fast paths can embed its backing address.
	newTableCaches []newTableCacheEntry

	// skipStandardDirectEntry lets a custom leaf emitter publish its own
	// t2_direct_entry without colliding with the standard full-frame entry.
	skipStandardDirectEntry bool

	// nativeCallReplaySafe is false when direct/native callers must not enter
	// this function because a callee exit would replay visible side effects.
	nativeCallReplaySafe bool

	// nativeCallCalleeResumeSafe is true when Tier 2 native callers can recover
	// from a callee exit by resuming the callee instead of replaying the call.
	nativeCallCalleeResumeSafe bool

	// rawIntSelfABI is the explicit private numeric self-recursive entry
	// descriptor for this compile. It is the source of truth for raw self
	// call emission during pass 2.
	rawIntSelfABI RawIntSelfABI

	// typedSelfABI describes the private typed self-recursive entry for
	// recursive table/int specializations that are not pure raw-int numeric specializations.
	typedSelfABI TypedSelfABI

	// entryShapeGuards are callee-entry table shape guards keyed by parameter
	// index. Every path that reaches the optimized body must either execute
	// these guards or fall back to the boxed VM call path first.
	entryShapeGuards map[int]FixedShapeTableFact

	// numericParamCount (R124) is set at emitContext construction when
	// the proto qualifies (qualifyForNumeric). Non-zero → Compile emits
	// an additional numeric body (pass 2) with the entry label
	// `t2_numeric_self_entry_N`.
	numericParamCount int

	// numericMode is set to true during pass 2 (numeric variant emit).
	// When true, block labels are prefixed "num_" (via blockLabelFor),
	// parameter LoadSlot reads raw ABI registers, Return branches through
	// num_epilogue with raw X0, and eligible recursive calls use the
	// raw-int self ABI.
	numericMode bool

	// numericParamSlots (R126) is the set of VM register slots that
	// correspond to function parameters. Populated when numericParamCount
	// > 0. In pass 2, LoadSlot for these slots reads X0..X(N-1) instead
	// of loading boxed VM slots.
	numericParamSlots map[int]bool

	// fusedCond holds the condition code from the last fused comparison.
	// Set by emitIntCmp/emitFloatCmp when the comparison is in fusedCmps.
	fusedCond jit.Cond

	// fusedActive is true when the preceding comparison was fused and
	// emitBranch should use fusedCond + B.cc instead of TBNZ.
	fusedActive bool

	// fusedBitTestActive is true when the preceding boolean producer can be
	// consumed by the immediately-following Branch as a direct bit test.
	fusedBitTestActive bool
	fusedBitTestReg    jit.Reg
	fusedBitTestBit    int
	fusedBitTestZero   bool

	// instrCodeRanges records the machine-code byte range emitted for each IR
	// instruction. It is diagnostic metadata only; offsets are relative to the
	// start of the compiled code block.
	instrCodeRanges []InstrCodeRange

	// exitResumeCheck carries debug-only site metadata and enables shadow
	// materialization writes when LEIA_EXIT_RESUME_CHECK=1 at compile time.
	exitResumeCheck *exitResumeCheckMetadata

	tier2BlockCounterIndex map[int]int
	tier2BlockCounterMeta  []Tier2BlockCounterMeta
	tier2BlockCounters     []uint64
	tier2CallCounterMeta   []Tier2CallCounterMeta
	tier2CallCounters      []uint64
}

// frameSize is the stack frame size for callee-saved registers.
const frameSize = 128

// numericSelfEntryFrameSize is the thin raw-int self-recursive frame. Raw
// callers preserve their own live allocated registers, so the numeric entry
// only needs FP/LR for the native BL/RET chain.
const numericSelfEntryFrameSize = 16
