//go:build darwin && arm64

package methodjit

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/never-labs/leia/internal/vm"
)

type tier2CompileDelayError struct {
	reason string
}

func (e tier2CompileDelayError) Error() string {
	return e.reason
}

func newTier2CompileDelayError(reason string) error {
	return tier2CompileDelayError{reason: reason}
}

func isTier2CompileDelayError(err error) bool {
	var delay tier2CompileDelayError
	return errors.As(err, &delay)
}

func (tm *TieringManager) compileTier2(proto *vm.FuncProto) (cf *CompiledFunction, retErr error) {
	started := time.Now()
	tm.tier2Attempts++
	attempt := tm.tier2Attempts
	tm.traceEvent("tier2_attempt", "tier2", proto, map[string]any{
		"attempt":    attempt,
		"call_count": proto.CallCount,
	})
	trace := tm.warmDumpTrace(proto)
	recordedWarmDump := false
	if tm.envR154Trace {
		fmt.Fprintf(os.Stderr, "[R154] compileTier2 ENTER proto=%q attempts=%d\n",
			proto.Name, tm.tier2Attempts)
		defer fmt.Fprintf(os.Stderr, "[R154] compileTier2 EXIT  proto=%q err=%v\n",
			proto.Name, retErr)
	}
	defer func() {
		if r := recover(); r != nil {
			cf = nil
			retErr = fmt.Errorf("tier2: panic during compilation: %v", r)
			if os.Getenv("LEIA_JIT_DEBUG") == "1" {
				fmt.Fprintf(os.Stderr, "tier2: panic during compilation of %q: %v\n", proto.Name, r)
			}
		}
		durationNanos := int64(time.Since(started))
		if retErr != nil {
			if isTier2CompileDelayError(retErr) {
				tm.traceEvent("tier2_defer", "tier2", proto, map[string]any{
					"attempt":                attempt,
					"reason":                 retErr.Error(),
					"compile_duration_nanos": durationNanos,
				})
				if trace != nil && !recordedWarmDump {
					tm.recordWarmDumpCompile(proto, trace, cf, retErr)
				}
				return
			}
			tm.markTier2Failed(proto, retErr.Error())
			tm.traceEvent("tier2_fail", "tier2", proto, map[string]any{
				"attempt":                attempt,
				"reason":                 retErr.Error(),
				"compile_duration_nanos": durationNanos,
			})
			if trace != nil && !recordedWarmDump {
				tm.recordWarmDumpCompile(proto, trace, cf, retErr)
			}
			if os.Getenv("LEIA_JIT_DEBUG") == "1" {
				fmt.Fprintf(os.Stderr, "tier2: compilation failed for %q: %v\n", proto.Name, retErr)
			}
		} else if os.Getenv("LEIA_JIT_DEBUG") == "1" {
			if cf != nil {
				cf.CompileDurationNanos = durationNanos
			}
			tm.traceTier2Success(proto, cf, attempt)
			fmt.Fprintf(os.Stderr, "tier2: compiled %q\n", proto.Name)
		} else {
			if cf != nil {
				cf.CompileDurationNanos = durationNanos
			}
			tm.traceTier2Success(proto, cf, attempt)
		}
	}()

	cf, retErr = tm.compileTier2Pipeline(proto, trace)
	if tm.envR154Trace && cf != nil && retErr == nil {
		codeSize := 0
		if cf.Code != nil {
			codeSize = cf.Code.Size()
		}
		fmt.Fprintf(os.Stderr, "[R154] compileTier2 RESUME proto=%q codeSize=%d resume=%v numericResume=%v\n",
			proto.Name, codeSize, cf.ResumeAddrs, cf.NumericResumeAddrs)
	}
	if trace != nil {
		tm.recordWarmDumpCompile(proto, trace, cf, retErr)
		recordedWarmDump = true
	}
	return cf, retErr
}

// CompileTier2 explicitly compiles a function at Tier 2. This bypasses the
// call count threshold and is useful for testing or when the caller knows
// the function is hot. Returns error if Tier 2 compilation fails.
func (tm *TieringManager) CompileTier2(proto *vm.FuncProto) error {
	if _, ok := tm.tier2CompiledFor(proto); ok {
		return nil // already compiled
	}
	if proto.Feedback == nil {
		proto.EnsureFeedback()
	}
	tm.ensureNativeLoopCallees(proto)
	tm.ensureRawIntLoopCallees(proto)
	if abi := AnalyzeTypedSelfABI(proto); abi.Eligible {
		t2, err := tm.compileTier2(proto)
		if err != nil {
			if !isTier2CompileDelayError(err) {
				tm.markTier2Failed(proto, err.Error())
			}
			return err
		}
		tm.markTier2Compiled(proto, t2)
		return nil
	}
	if t2, ok := tm.compileTier2RuntimeSpecializationEntry(proto); ok {
		tm.markTier2Compiled(proto, t2)
		return nil
	}
	t2, err := tm.compileTier2(proto)
	if err != nil {
		if !isTier2CompileDelayError(err) {
			tm.markTier2Failed(proto, err.Error())
		}
		return err
	}
	tm.markTier2Compiled(proto, t2)

	return nil
}

// compileTier2Pipeline is the pure pipeline body shared between production
// compileTier2 and CompileForDiagnostics. It performs NO bookkeeping
// (counters, fail-reason maps, debug logging) so diagnostic calls cannot
// contaminate production state. It DOES mutate proto.NeedsTier2 and
// proto.MaxStack when the optimized function requires it — both are part of
// production compilation semantics and must be preserved identically so the
// diagnostic path is bit-identical to production.
//
// trace is optional. When non-nil, intermediate artifacts are captured into
// it for the diagnostic caller. When nil, the pipeline runs without
// observation overhead.
//
// Any change to this function's body is a change to the production Tier 2
// compile semantics AND to what the diagnostic tool sees, by construction.
// That is the load-bearing invariant of rule 5 in CLAUDE.md.
func (tm *TieringManager) compileTier2Pipeline(proto *vm.FuncProto, trace *Tier2Trace) (*CompiledFunction, error) {
	speculation := NewTier2SpeculationPlanWithSuppressedGuardKinds(proto, tm.tier2SuppressedGuards(proto), tm.tier2SuppressedGuardKinds(proto))
	var remarks *OptimizationRemarks
	if trace != nil {
		trace.Specialization = speculation.Summary()
		remarks = &OptimizationRemarks{}
		defer func() {
			trace.OptimizationRemarks = remarks.List()
		}()
	}
	stages := make([]tier2CompileStage, 0, 9)
	addStage := func(name string, body func() error) {
		stages = append(stages, tier2CompileStage{
			name: name,
			run:  body,
		})
	}

	addStage("Tier2Gate", func() error {
		if gate := firstUnsupportedTier2BytecodeGate(proto); !gate.Allowed {
			if gate.Reason != "" {
				remarks.Add("Tier2Gate", "blocked", 0, 0, OpNop,
					fmt.Sprintf("unsupported bytecode %s", gate.Reason))
			} else {
				remarks.Add("Tier2Gate", "blocked", 0, 0, OpNop,
					"function has unsupported ops")
			}
			return fmt.Errorf("tier2: function has unsupported ops, staying at tier 1")
		}
		return nil
	})

	var fn *Function
	addStage("BuildGraph", func() error {
		fn = BuildGraphWithSpeculation(proto, speculation)
		fn.Remarks = remarks
		if trace != nil {
			trace.IRBefore = Print(fn)
		}
		if fn.Unpromotable {
			remarks.Add("Tier2Gate", "blocked", 0, 0, OpNop,
				"BuildGraph marked function unpromotable")
			return fmt.Errorf("tier2: function uses unmodeled bytecode (variadic CALL), staying at Tier 1")
		}
		return nil
	})

	addStage("ValidateInitialIR", func() error {
		if errs := Validate(fn); len(errs) > 0 {
			remarks.Add("Tier2Gate", "blocked", 0, 0, OpNop,
				"initial IR validation failed: "+errs[0].Error())
			return fmt.Errorf("tier2: validation failed: %v", errs[0])
		}
		if gate := readWriteGlobalInSameLoopGate(fn); !gate.Allowed {
			if hasIndexedGlobalLoopProtocol(fn) && analyzeFuncProfile(proto).CallCount > 0 {
				remarks.Add("Tier2Gate", "changed", 0, 0, gate.Op,
					"read/write global accepted by indexed native global protocol")
				return nil
			}
			remarks.Add("Tier2Gate", "blocked", 0, 0, gate.Op, gate.Reason)
			return fmt.Errorf("tier2: %s, staying at Tier 1", gate.Reason)
		}
		if typedABIHasStaticSelfCall(proto) && protoReturnsOnlyNoResults(proto) {
			remarks.Add("Tier2Gate", "blocked", 0, 0, OpCall,
				"zero-result self recursion is not native-call safe")
			return fmt.Errorf("tier2: zero-result self recursion is not native-call safe, staying at Tier 1")
		}
		if typedABIHasStaticSelfCall(proto) && protoHasRecursiveTableSurface(proto) {
			if abi := AnalyzeTypedSelfABI(proto); !abi.Eligible {
				remarks.Add("Tier2Gate", "blocked", 0, 0, OpCall,
					"recursive table function lacks a proven typed self ABI: "+abi.RejectWhy)
				return fmt.Errorf("tier2: recursive table function lacks a proven typed self ABI, staying at Tier 1")
			}
		}
		return nil
	})

	var inlineGlobals map[string]*vm.FuncProto
	var loopCallGlobals map[string]*vm.FuncProto
	var opts *Tier2PipelineOpts
	dependencyRegistry := NewCompilationDependencyRegistry()
	var optimizerTimings []PipelineStageTiming
	var moduleRuns []Tier2ModuleRun
	optimizerTimingsFlushed := false
	flushOptimizerTimings := func() {
		if trace == nil || optimizerTimingsFlushed || len(optimizerTimings) == 0 {
			return
		}
		trace.PipelineStages = append(trace.PipelineStages, optimizerTimings...)
		optimizerTimingsFlushed = true
	}
	addStage("BuildPipelineOptions", func() error {
		inlineGlobals = tm.buildInlineGlobals()
		loopCallGlobals = inlineGlobals
		loopCallGlobalsOwned := false
		if protoGlobals := buildProtoInlineGlobals(proto); len(protoGlobals) > 0 {
			loopCallGlobals = make(map[string]*vm.FuncProto, len(inlineGlobals)+len(protoGlobals))
			loopCallGlobalsOwned = true
			for name, calleeProto := range inlineGlobals {
				loopCallGlobals[name] = calleeProto
			}
			for name, calleeProto := range protoGlobals {
				if _, ok := loopCallGlobals[name]; !ok {
					loopCallGlobals[name] = calleeProto
				}
			}
		}
		if stableGlobals := buildProtoStableGlobals(proto); len(stableGlobals) > 0 {
			if !loopCallGlobalsOwned {
				loopCallGlobals = make(map[string]*vm.FuncProto, len(inlineGlobals)+len(stableGlobals))
				loopCallGlobalsOwned = true
				for name, calleeProto := range inlineGlobals {
					loopCallGlobals[name] = calleeProto
				}
			}
			for name, calleeProto := range stableGlobals {
				if _, ok := loopCallGlobals[name]; !ok {
					loopCallGlobals[name] = calleeProto
				}
			}
		}
		staticArrayElementFacts := inferGuardedFixedShapeArrayElementArgFactsForProto(proto, loopCallGlobals)
		staticArgFacts := inferGuardedFixedShapeArgFactsForProto(proto, loopCallGlobals)
		profiledArgFacts := profiledFixedShapeArgFactsForProto(proto)
		profiledArrayElementFacts := profiledFixedShapeArrayElementArgFactsForProto(proto)
		if conflicts := guardedFixedShapeArgConflictParamsForProto(proto, loopCallGlobals); len(conflicts) > 0 && len(profiledArgFacts) > 0 {
			profiledArgFacts = cloneFixedShapeTableFactIntMap(profiledArgFacts)
			for paramIdx := range conflicts {
				delete(profiledArgFacts, paramIdx)
			}
		}
		if conflicts := guardedFixedShapeArrayElementArgConflictParamsForProto(proto, loopCallGlobals); len(conflicts) > 0 && len(profiledArrayElementFacts) > 0 {
			profiledArrayElementFacts = cloneFixedShapeTableFactIntMap(profiledArrayElementFacts)
			for paramIdx := range conflicts {
				delete(profiledArrayElementFacts, paramIdx)
			}
		}
		profiledArrayElementPolyFacts := profiledFixedShapeArrayElementPolyFactsForProto(proto)
		profiledArgPolyFacts := profiledFixedShapeArgPolyFactsForProto(proto)
		opts = &Tier2PipelineOpts{
			InlineGlobals:                   inlineGlobals,
			SpecializationGlobals:           loopCallGlobals,
			GlobalConstValues:               tm.buildNumericGlobalConstValues(proto),
			InlineMaxSize:                   inlineMaxCalleeSize,
			FixedShapeArgFacts:              mergeFixedShapeTableFacts(profiledArgFacts, staticArgFacts),
			FixedShapeArgPolyFacts:          profiledArgPolyFacts,
			FixedShapeArrayElementArgFacts:  mergeFixedShapeTableFacts(profiledArrayElementFacts, staticArrayElementFacts),
			FixedShapeArrayElementPolyFacts: profiledArrayElementPolyFacts,
			GlobalArrayElementFacts:         tm.stableGlobalArrayElementFacts(),
			FixedShapeEntryGuards:           true,
			ForceBoxIntIDs:                  tm.forcedBoxTier2IntValues(proto),
			Remarks:                         remarks,
			DependencyRegistry:              dependencyRegistry,
			DependencyContext:               CompilationDependencyContext{Globals: tm.callVM},
		}
		if trace != nil {
			opts.OptimizerTimings = &optimizerTimings
			opts.ModuleRuns = &moduleRuns
		}
		return nil
	})

	var intrinsicNotes []string
	addStage("RunTier2Pipeline", func() error {
		var err error
		fn, intrinsicNotes, err = RunTier2Pipeline(fn, opts)
		flushOptimizerTimings()
		if trace != nil {
			trace.ModuleRuns = append([]Tier2ModuleRun(nil), moduleRuns...)
		}
		if err != nil {
			if trace != nil && trace.IRAfter == "" {
				if lastFn := lastTier2ModuleRunFunction(moduleRuns); lastFn != nil {
					trace.IRAfter = Print(lastFn)
				}
			}
			remarks.Add("Tier2Gate", "blocked", 0, 0, OpNop,
				"optimization pipeline failed: "+err.Error())
			return fmt.Errorf("tier2: pipeline: %w", err)
		}
		globalFacts := functionGlobalFacts(fn)
		tm.learnGlobalNumericFacts(globalFacts.NumericGlobalValuesMap())
		tm.learnGlobalArrayElementFacts(globalFacts.GlobalArrayElementFactsMap())
		return nil
	})

	addStage("PostPipelineGates", func() error {
		if len(intrinsicNotes) > 0 {
			proto.NeedsTier2 = true
		}
		if shouldStayTier1ForBoxedRawIntSpecialization(proto, analyzeFuncProfile(proto)) {
			forceRawIntSpecializationIR(fn)
			if gate := firstResidualRawIntSpecializationGenericNumericGate(fn); !gate.Allowed {
				remarks.Add("Tier2Gate", "blocked", 0, 0, gate.Op,
					fmt.Sprintf("%s %s", gate.Reason, gate.Op))
				return fmt.Errorf("tier2: %s %s, staying at Tier 1", gate.Reason, gate.Op)
			}
		}
		if gate := firstUnsupportedHighArityCallResultShapeInLoopGate(fn); !gate.Allowed {
			remarks.Add("Tier2Gate", "blocked", 0, 0, gate.Op, gate.Reason)
			return fmt.Errorf("tier2: %s, staying at Tier 1", gate.Reason)
		}
		if call, ok := firstResidualFieldCalleeCallInLoop(fn); ok {
			remarks.Add("Tier2Gate", "blocked", call.Block.ID, call.ID, OpCall,
				"loop residual GetField callee call needs table-exit resume")
			return fmt.Errorf("tier2: loop residual GetField callee call, staying at Tier 1")
		}
		fn.CarryPreheaderInvariants = true
		if trace != nil {
			trace.IRAfter = Print(fn)
			trace.IntrinsicNotes = intrinsicNotes
		}
		if gate := firstSelfRecursiveTableMutationInLoopGate(fn); !gate.Allowed {
			remarks.Add("Tier2Gate", "blocked", 0, 0, gate.Op,
				fmt.Sprintf("%s %s", gate.Reason, gate.Op))
			return fmt.Errorf("tier2: %s %s (exit-storm blocked), staying at Tier 1", gate.Reason, gate.Op)
		}
		if gate := firstTier2ModBlockerInLoopGate(fn); !gate.Allowed {
			if !shouldStayTier1ForBoxedRawIntSpecialization(proto, analyzeFuncProfile(proto)) {
				remarks.Add("Tier2Gate", "blocked", 0, 0, OpMod,
					gate.Reason+" remains inside loop")
				return fmt.Errorf("tier2: has %s (performance-blocked), staying at Tier 1", gate.Reason)
			}
		}

		// R162/R171: reject Tier 2 promotion when a loop contains operations
		// whose Tier 2 path is still expected to be slower than Tier 1. This is
		// deliberately a call-boundary performance filter, not the restart-OSR
		// correctness filter: functions compiled before entering bytecode PC 0 do
		// not replay partially executed table mutations. Restart-style OSR remains
		// gated by isOSRRestartSafe before the OSR counter is armed.
		//
		// Bypass via LEIA_TIER2_NO_FILTER=1 (diagnostic / perf-comparison).
		//
		// Depth-aware filter (R162): old LoopDepth>=2 candidates use the classic
		// non-native-call filter. LoopDepth<2 candidates use the stricter blocker
		// list below, but read-only OpGetTable is allowed because Tier 2 has native
		// int-key table fast paths plus table-exit resume metadata for misses.
		// Table writes that can resize/mutate dynamic structure, residual
		// allocations, and non-native calls are still blocked by default.
		if !tm.envTier2NoFilter {
			profile := tm.getProfile(proto)
			if profile.LoopDepth < 2 {
				if gate := readWriteGlobalInSameLoopGate(fn); !gate.Allowed {
					if !hasIndexedGlobalLoopProtocol(fn) || profile.CallCount == 0 {
						remarks.Add("Tier2Gate", "blocked", 0, 0, gate.Op,
							"LoopDepth<2 candidate "+gate.Reason)
						return fmt.Errorf("tier2: LoopDepth<2 candidate has read/write global state inside loop, staying at Tier 1")
					}
					remarks.Add("Tier2Gate", "changed", 0, 0, OpSetGlobal,
						"LoopDepth<2 read/write globals accepted by indexed native global protocol")
				}
				if gate := firstCallBoundaryTier2BlockerInLoopGate(fn, loopCallGlobals); !gate.Allowed {
					remarks.Add("Tier2Gate", "blocked", 0, 0, gate.Op,
						fmt.Sprintf("LoopDepth<2 candidate has performance-blocked %s inside loop", gate.Op))
					return fmt.Errorf("tier2: LoopDepth<2 candidate has performance-blocked op %s inside loop, staying at Tier 1", gate.Op)
				}
			} else {
				if hasBlockingNonNativeCallInLoop(fn, loopCallGlobals) {
					remarks.Add("Tier2Gate", "blocked", 0, 0, OpCall,
						"non-native OpCall remains inside loop after inlining")
					return fmt.Errorf("tier2: has OpCall inside loop (performance-blocked), staying at Tier 1")
				}
				if gate := firstTableArrayObjectFieldLoadBlockerGate(fn); !gate.Allowed {
					remarks.Add("Tier2Gate", "blocked", 0, 0, gate.Op,
						gate.Reason)
					return fmt.Errorf("tier2: %s, staying at Tier 1", gate.Reason)
				}
			}
			if gate := firstLoopCarriedObjectGraphBlockerGate(fn); !gate.Allowed {
				remarks.Add("Tier2Gate", "blocked", 0, 0, gate.Op,
					fmt.Sprintf("candidate has unsupported loop-carried object graph mutation %s", gate.Op))
				if tier2LoopCarriedObjectGraphNeedsRuntimeFeedback(fn) {
					return newTier2CompileDelayError(fmt.Sprintf("tier2: loop-carried object graph mutation %s needs runtime shape feedback, deferring Tier 2", gate.Op))
				}
				return fmt.Errorf("tier2: has loop-carried object graph mutation %s (unsupported), staying at Tier 1", gate.Op)
			}
		}

		// R40: mark Proto.HasSelfCalls so the emitter opts in to the
		// t2_self_entry lightweight path. A self-call is an OpCall whose
		// function argument comes from an OpGetGlobal loading this proto's
		// own name.
		proto.LeafNoCall = protoHasNoCallLikeOps(proto)
		proto.Tier2LeafNoCall = !irHasNestedCallLike(fn)
		proto.NoGlobalOps = protoHasNoGlobalOps(proto)
		if irHasSelfCall(fn) {
			proto.HasSelfCalls = true
		}
		return nil
	})

	var alloc *RegAllocation
	addStage("RegAlloc", func() error {
		alloc = AllocateRegisters(fn)
		if trace != nil {
			trace.RegAllocMap = formatRegAlloc(alloc)
			trace.LoopDiagnostics = BuildLoopDiagnostics(fn, alloc)
		}
		return nil
	})

	var cf *CompiledFunction
	addStage("ARM64Compile", func() error {
		var err error
		traceNativeCalls := tm.envR154Trace || trace != nil || os.Getenv("LEIA_JIT_DEBUG") == "1"
		cf, err = CompileWithOptions(fn, alloc, CompileOptions{
			EnableTier2BlockCounters: tm.perfStatsEnabled,
			TraceNativeCalls:         traceNativeCalls,
			PrintNativeCallTrace:     tm.envR154Trace || os.Getenv("LEIA_JIT_DEBUG") == "1",
		})
		if err != nil {
			remarks.Add("Tier2Gate", "blocked", 0, 0, OpNop,
				"ARM64 compile failed: "+err.Error())
			return fmt.Errorf("tier2: compile failed: %w", err)
		}
		cf.SpeculationSnapshot = speculation.Snapshot
		cf.SpecializationVersion = speculation.Profile.Version
		cf.SpecDependencyProtos = sortedSpecDependencyProtos(fn)
		cf.CompilationDependencies = dependencyRegistry.Dependencies()
		return nil
	})
	if trace != nil {
		addStage("SourceMap", func() error {
			trace.SourceMap = BuildIRASMMap(fn, cf.InstrCodeRanges)
			return nil
		})
	}

	if err := runTier2CompileStages(trace, stages); err != nil {
		return nil, err
	}

	rawIntSelfABI := AnalyzeRawIntSelfABI(proto)
	cf.RawIntSelfABI = rawIntSelfABI
	cf.NumericParamCount = rawIntSelfABI.NumParams

	if cf.numRegs > proto.MaxStack {
		proto.MaxStack = cf.numRegs
	}
	if reserve := typedPeerCallRegisterReserve(fn, cf.numRegs); reserve > cf.numRegs {
		cf.numRegs = reserve
		proto.MaxStack = reserve
	}

	// R124: The numeric entry (t2_numeric_self_entry_N) is emitted as
	// an extra label at the end of the same code block when the proto
	// qualifies, so caller BL is compile-time PC-relative.
	return cf, nil
}

func lastTier2ModuleRunFunction(runs []Tier2ModuleRun) *Function {
	for i := len(runs) - 1; i >= 0; i-- {
		if runs[i].Function != nil {
			return runs[i].Function
		}
	}
	return nil
}

func typedPeerCallRegisterReserve(fn *Function, baseSlots int) int {
	callFacts := functionCallFacts(fn)
	if callFacts.CallABICount() == 0 {
		return 0
	}
	reserve := baseSlots
	callFacts.ForEachCallABI(func(_ int, desc CallABIDescriptor) bool {
		if !desc.TypedPeer || desc.Callee == nil {
			return true
		}
		needed := baseSlots + desc.Callee.MaxStack + 1
		if needed > reserve {
			reserve = needed
		}
		return true
	})
	return reserve
}
