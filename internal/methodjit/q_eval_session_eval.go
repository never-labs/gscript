package methodjit

// q_eval_session_eval.go lowers constant-source q session eval calls inside
// Tier 2 candidates to OpQEvalSessionEval, a result-producing op-exit.
//
// Recognized shape (per call site, no whole-session analysis required):
//
//	qs := q.session()        // OpCall, callee = GetField "session" on global q
//	... qs.eval(<const str>) // OpCall, callee = GetField "eval" on qs
//
// The lowered op keeps the *receiver session value* as its single argument and
// the constant source as Aux (constant pool index). At runtime the op-exit
// handler executes through one of two routes:
//
//   - planned (preferred): the first execution per (exit site, receiver
//     session) resolves the session's pinned plan chain for the constant
//     source via the reserved stdq.SessionPlannedEvalField resolver that the
//     bind layer installs on session tables; subsequent iterations execute
//     the pinned chain directly, skipping the per-iteration string-eval shell
//     (host eval field lookup + TrimSpace + source-string plan-cache probe)
//     while running the exact same cached plans against the same EvalState.
//   - shell (fallback): receivers without the resolver go through the
//     receiver's own "eval" host function with the constant source value —
//     exactly what the generic call path does.
//
// Both routes preserve session state, q plan caching, and result/error
// semantics for every constant source, and both re-execute the typed q
// kernels per iteration (no result memoization). The exit itself is emitted
// with selective spill (emit_q_eval_session_eval.go): only values live across
// the exit are spilled/reloaded, mirroring the OpQEvalPipelinePlan native
// exit. Unlike OpQEvalPipelinePlan (zero-arg, pure-read, limited to
// typed-runtime describable sources), this op:
//
//   - covers any constant q source the session evaluator accepts,
//   - is effectful (OpSideEffectCall): never hoisted/CSE'd, so a hot loop
//     re-evaluates per iteration (q.session.eval has no result cache;
//     EvalSession caches parse/plan artifacts only), and
//   - removes the per-iteration generic Call from the loop, which lifts the
//     Tier 2 "performance-blocked op Call inside loop" gate for loops whose
//     only call was the recognized session eval.
//
// Sources that fail stdq.EvalSourceCacheable (system/file/handle effects) are
// deliberately left on the generic call path: an op-exit error falls back by
// re-executing the function from the top, and replaying such effects would be
// observable. Cacheable q sources are deterministic, so the fallback replay
// hazard matches the existing q op-exit family.

import (
	"fmt"
	"sync/atomic"

	"github.com/never-labs/leia/internal/runtime"
	stdq "github.com/never-labs/leia/internal/stdlib/lib/q"
	"github.com/never-labs/leia/internal/vm"
)

// qEvalSessionEvalExecutionCounters keeps lock-free per-CompiledFunction
// success/error counters for the q session eval op-exit, split by execution
// route: planned (pinned session plan chain, no string-eval shell) versus
// shell (per-call host eval function). The exit fires once per loop
// iteration, so the hot path must not take the qKernelStats mutex;
// QKernelExecutionStats folds these counters back into the public stat rows.
type qEvalSessionEvalExecutionCounters struct {
	success        atomic.Uint64
	errors         atomic.Uint64
	plannedSuccess atomic.Uint64
	plannedErrors  atomic.Uint64
}

func (cf *CompiledFunction) recordQEvalSessionEvalExecution(err error) {
	if cf == nil {
		return
	}
	if err != nil {
		cf.QEvalSessionEvalStats.errors.Add(1)
		return
	}
	cf.QEvalSessionEvalStats.success.Add(1)
}

func (cf *CompiledFunction) recordQEvalSessionEvalPlannedExecution(err error) {
	if cf == nil {
		return
	}
	if err != nil {
		cf.QEvalSessionEvalStats.plannedErrors.Add(1)
		return
	}
	cf.QEvalSessionEvalStats.plannedSuccess.Add(1)
}

func (cf *CompiledFunction) appendQEvalSessionEvalExecutionStats(out map[qKernelExecutionKey]uint64) {
	if cf == nil {
		return
	}
	appendQEvalSessionEvalCounter(out, qEvalSessionEvalRouteShell, "success", cf.QEvalSessionEvalStats.success.Load())
	appendQEvalSessionEvalCounter(out, qEvalSessionEvalRouteShell, "error", cf.QEvalSessionEvalStats.errors.Load())
	appendQEvalSessionEvalCounter(out, qEvalSessionEvalRoutePlanned, "success", cf.QEvalSessionEvalStats.plannedSuccess.Load())
	appendQEvalSessionEvalCounter(out, qEvalSessionEvalRoutePlanned, "error", cf.QEvalSessionEvalStats.plannedErrors.Load())
}

const (
	// qEvalSessionEvalRouteShell labels per-iteration executions that went
	// through the session's host eval function (string-eval shell).
	qEvalSessionEvalRouteShell = "typed_runtime_op_exit"
	// qEvalSessionEvalRoutePlanned labels per-iteration executions of the
	// pinned session plan chain (resolved once per receiver+source).
	qEvalSessionEvalRoutePlanned = "session_planned_op_exit"
)

func appendQEvalSessionEvalCounter(out map[qKernelExecutionKey]uint64, route, outcome string, count uint64) {
	if count == 0 {
		return
	}
	out[qKernelExecutionKey{
		source:        "methodjit_q_eval_runtime",
		kernel:        "QEvalSessionEval",
		shape:         "q-eval/session-eval",
		pipelineShape: "unknown",
		route:         route,
		outcome:       outcome,
		reasonCode:    qKernelExecutionReasonCode(outcome, ""),
	}] += count
}

// qCallIsLowerableQSessionEval reports whether a call instruction matches the
// full lowerable shape: session receiver, constant cacheable source.
func qCallIsLowerableQSessionEval(fn *Function, call *Instr) bool {
	if call == nil || call.Op != OpCall {
		return false
	}
	if _, ok := qCallSessionEvalReceiver(fn, call); !ok {
		return false
	}
	_, source, ok := qCallEvalSourceConstIndex(fn, call)
	return ok && stdq.EvalSourceCacheable(source)
}

// protoLoopCallsAreLowerableQSessionEval reports whether every call-like op
// inside the proto's loops is a recognizable constant-source q session eval
// (and at least one exists). Tier promotion policy uses this to keep such
// loops eligible for Tier 2: the calls disappear during the Tier 2 pipeline
// (QEvalSessionEvalLoweringPass), so bytecode-level "call inside loop"
// heuristics would otherwise misclassify the loop as a generic host-call loop
// and pin it at Tier 0/Tier 1.
func protoLoopCallsAreLowerableQSessionEval(proto *vm.FuncProto) bool {
	fn := BuildGraph(proto)
	if fn == nil || fn.Entry == nil || fn.Unpromotable {
		return false
	}
	li := computeLoopInfo(fn)
	found := false
	for _, block := range fn.Blocks {
		if block == nil || !li.loopBlocks[block.ID] {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil || !tier2LoopCallOp(instr.Op) {
				continue
			}
			if qCallIsLowerableQSessionEval(fn, instr) {
				found = true
				continue
			}
			return false
		}
	}
	return found
}

// QEvalSessionEvalLoweringPass rewrites recognized constant-source q session
// eval calls to OpQEvalSessionEval. Dynamic sources, non-session receivers,
// and uncacheable sources remain generic OpCall fallbacks.
func QEvalSessionEvalLoweringPass(fn *Function) (*Function, error) {
	if fn == nil {
		return fn, nil
	}
	changed := false
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpCall {
				continue
			}
			receiver, ok := qCallSessionEvalReceiver(fn, instr)
			if !ok {
				continue
			}
			srcIdx, source, ok := qCallEvalSourceConstIndex(fn, instr)
			if !ok {
				continue
			}
			if !stdq.EvalSourceCacheable(source) {
				blockID, valueID := qRemarkLocation(instr)
				functionRemarks(fn).AddWithFields("QEvalSessionEvalLowering", "missed", blockID, valueID, OpCall,
					"constant q session eval source is not cacheable; staying on generic call path",
					map[string]string{
						"kind":        "fallback",
						"kernel":      "QEvalSessionEval",
						"shape":       "q-eval/session-eval",
						"reason_code": "uncacheable_source",
						"route":       "lowering",
						"outcome":     "fallback",
					})
				continue
			}
			instr.Op = OpQEvalSessionEval
			instr.Type = TypeAny
			instr.Args = []*Value{receiver}
			instr.Aux = int64(srcIdx)
			instr.Aux2 = 0
			changed = true
			blockID, valueID := qRemarkLocation(instr)
			functionRemarks(fn).AddWithFields("QEvalSessionEvalLowering", "changed", blockID, valueID, OpQEvalSessionEval,
				"lowered constant-source q session eval call to per-iteration session eval op-exit",
				map[string]string{
					"kind":    "runtime_kernel",
					"kernel":  "QEvalSessionEval",
					"shape":   "q-eval/session-eval",
					"route":   "session_eval_op",
					"outcome": "lowered",
				})
		}
	}
	if changed {
		qEvalSessionEvalNopDeadEvalFields(fn)
	}
	return fn, nil
}

// qCallSessionEvalReceiver matches OpCall instructions whose callee is a
// GetField "eval" load from a value produced directly by a zero-arg
// q.session() / q.workspace() call. It returns the session receiver value.
func qCallSessionEvalReceiver(fn *Function, call *Instr) (*Value, bool) {
	if fn == nil || call == nil || len(call.Args) == 0 || call.Args[0] == nil {
		return nil, false
	}
	callee := call.Args[0].Def
	if callee == nil || callee.Op != OpGetField || len(callee.Args) != 1 || callee.Args[0] == nil {
		return nil, false
	}
	field, ok := qConstStringAt(fn, int(callee.Aux))
	if !ok || field != "eval" {
		return nil, false
	}
	receiver := callee.Args[0]
	if !qValueIsQSessionCall(fn, receiver) {
		return nil, false
	}
	return receiver, true
}

// qValueIsQSessionCall reports whether v is the direct result of a zero-arg
// q.session() or q.workspace() call on the global q table. Values flowing
// through phis or other ops are conservatively rejected.
func qValueIsQSessionCall(fn *Function, v *Value) bool {
	if v == nil || v.Def == nil {
		return false
	}
	sessionCall := v.Def
	if sessionCall.Op != OpCall || len(sessionCall.Args) != 1 || sessionCall.Args[0] == nil {
		return false
	}
	sessionCallee := sessionCall.Args[0].Def
	if sessionCallee == nil || sessionCallee.Op != OpGetField || len(sessionCallee.Args) != 1 || sessionCallee.Args[0] == nil {
		return false
	}
	field, ok := qConstStringAt(fn, int(sessionCallee.Aux))
	if !ok || (field != "session" && field != "workspace") {
		return false
	}
	global := sessionCallee.Args[0].Def
	if global == nil || global.Op != OpGetGlobal {
		return false
	}
	name, ok := qConstStringAt(fn, int(global.Aux))
	return ok && name == "q"
}

// qCallEvalSourceConstIndex returns the constant pool index and text of the
// single constant string argument of an eval call.
func qCallEvalSourceConstIndex(fn *Function, call *Instr) (int, string, bool) {
	if fn == nil || call == nil || len(call.Args) != 2 || call.Args[1] == nil {
		return 0, "", false
	}
	def := call.Args[1].Def
	if def == nil || def.Op != OpConstString {
		return 0, "", false
	}
	idx := int(def.Aux)
	source, ok := qConstStringAt(fn, idx)
	if !ok {
		return 0, "", false
	}
	return idx, source, true
}

// qEvalSessionEvalNopDeadEvalFields clears GetField "eval" loads that lost
// their last use to session eval lowering. DCE already ran by final_call, so
// without this the dead field load would stay in the loop body.
func qEvalSessionEvalNopDeadEvalFields(fn *Function) {
	uses := qQueryValueUseCounts(fn)
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpGetField || len(instr.Args) != 1 {
				continue
			}
			field, ok := qConstStringAt(fn, int(instr.Aux))
			if !ok || field != "eval" || uses[instr.ID] != 0 {
				continue
			}
			if !qValueIsQSessionCall(fn, instr.Args[0]) {
				continue
			}
			qQueryNop(instr)
		}
	}
}

// qEvalSessionEvalSite memoizes the resolved planned-eval executor for one
// lowered session-eval op-exit site. Sites are allocated at compile time (one
// per OpQEvalSessionEval instruction) and updated lock-free at runtime: the
// loaded pointer is only used when its receiver matches the current receiver
// table identity, so concurrent executions with different sessions stay
// correct (they re-resolve and overwrite each other without corruption).
type qEvalSessionEvalSite struct {
	planned atomic.Pointer[qEvalSessionEvalPlanned]
}

// qEvalSessionEvalPlanned binds a resolved planned-eval executor to the
// receiver session table it was resolved from. exec runs the session's pinned
// plan chain directly (its Value argument is ignored); it performs the same
// per-call typed-kernel work, locking, error wrapping, and value conversion
// as the host eval shell, minus the per-call source resolution.
type qEvalSessionEvalPlanned struct {
	receiver *runtime.Table
	exec     func(runtime.Value) (runtime.Value, error)
}

// qEvalSessionEvalSiteTable allocates one planned-executor cache per
// OpQEvalSessionEval instruction in fn, indexed by instruction ID (nil when
// the op is absent). Slice indexing keeps the per-iteration site probe a
// bounds check instead of a map lookup, matching the other QEval* id tables.
func qEvalSessionEvalSiteTable(fn *Function) []*qEvalSessionEvalSite {
	if fn == nil {
		return nil
	}
	maxID := -1
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr != nil && instr.Op == OpQEvalSessionEval && instr.ID > maxID {
				maxID = instr.ID
			}
		}
	}
	if maxID < 0 {
		return nil
	}
	sites := make([]*qEvalSessionEvalSite, maxID+1)
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpQEvalSessionEval {
				continue
			}
			sites[instr.ID] = &qEvalSessionEvalSite{}
		}
	}
	return sites
}

// qEvalSessionEvalSite returns the planned-executor cache for an
// OpQEvalSessionEval instruction ID (nil when absent).
func (cf *CompiledFunction) qEvalSessionEvalSite(instrID int) *qEvalSessionEvalSite {
	if cf == nil || instrID < 0 || instrID >= len(cf.QEvalSessionEvalSites) {
		return nil
	}
	return cf.QEvalSessionEvalSites[instrID]
}

// executeQEvalSessionEval is the op-exit entry for OpQEvalSessionEval. It
// prefers the planned route: the first execution per (site, receiver session)
// resolves the session's planned-eval handle for the constant source (via the
// reserved stdq.SessionPlannedEvalField resolver bind installs on session
// tables) and memoizes it; subsequent iterations execute the pinned plan
// chain directly, skipping the per-iteration string-eval shell (host eval
// field lookup + TrimSpace + source-string plan-cache probe). Sessions whose
// table does not expose the resolver fall back to the host eval shell, whose
// semantics the planned route matches exactly.
//
// Receiver-identity note: the lowering only accepts receivers produced
// directly by q.session()/q.workspace() in the same function (no phis), and
// the loop body containing this op cannot rebind the session's eval fields,
// so resolving the executor once per receiver is semantically equivalent to
// the shell's per-iteration field lookup.
func (cf *CompiledFunction) executeQEvalSessionEval(instrID, aux int, receiver runtime.Value) (runtime.Value, error) {
	var constants []runtime.Value
	if cf != nil && cf.Proto != nil {
		constants = cf.Proto.Constants
	}
	if cf != nil && receiver.IsTable() {
		if site := cf.qEvalSessionEvalSite(instrID); site != nil {
			tbl := receiver.Table()
			planned := site.planned.Load()
			if planned == nil || planned.receiver != tbl {
				if exec, ok := resolveQEvalSessionPlannedExec(tbl, constants, aux); ok {
					planned = &qEvalSessionEvalPlanned{receiver: tbl, exec: exec}
					site.planned.Store(planned)
				} else {
					planned = nil
				}
			}
			if planned != nil {
				out, err := planned.exec(runtime.NilValue())
				cf.recordQEvalSessionEvalPlannedExecution(err)
				return out, err
			}
		}
	}
	out, err := executeQEvalSessionEvalValue(constants, aux, receiver)
	cf.recordQEvalSessionEvalExecution(err)
	return out, err
}

// resolveQEvalSessionPlannedExec asks the receiver session table's reserved
// planned-eval resolver for a direct executor bound to the constant source.
func resolveQEvalSessionPlannedExec(tbl *runtime.Table, constants []runtime.Value, aux int) (func(runtime.Value) (runtime.Value, error), bool) {
	if tbl == nil || aux < 0 || aux >= len(constants) || !constants[aux].IsString() {
		return nil, false
	}
	resolver := tbl.RawGetString(stdq.SessionPlannedEvalField).GoFunction()
	if resolver == nil || resolver.FastArg1 == nil {
		return nil, false
	}
	handle, err := resolver.FastArg1(constants[aux])
	if err != nil {
		return nil, false
	}
	exec := handle.GoFunction()
	if exec == nil || exec.FastArg1 == nil {
		return nil, false
	}
	return exec.FastArg1, true
}

// executeQEvalSessionEvalValue runs one lowered session eval: it loads the
// receiver session's "eval" host function and invokes it with the constant
// source. The receiver is the runtime session table value, so state and error
// behavior match the generic call path exactly.
func executeQEvalSessionEvalValue(constants []runtime.Value, aux int, receiver runtime.Value) (runtime.Value, error) {
	if aux < 0 || aux >= len(constants) || !constants[aux].IsString() {
		return runtime.NilValue(), fmt.Errorf("methodjit: q session eval source constant %d out of range", aux)
	}
	source := constants[aux]
	if !receiver.IsTable() {
		return runtime.NilValue(), fmt.Errorf("methodjit: q session eval receiver is %s, not a session table", receiver.TypeName())
	}
	evalFn := receiver.Table().RawGetString("eval").GoFunction()
	if evalFn == nil {
		return runtime.NilValue(), fmt.Errorf("methodjit: q session eval receiver has no host eval function")
	}
	if evalFn.FastArg1 != nil {
		return evalFn.FastArg1(source)
	}
	if evalFn.Fn == nil {
		return runtime.NilValue(), fmt.Errorf("methodjit: q session eval host function is not callable")
	}
	out, err := evalFn.Fn([]runtime.Value{source})
	if err != nil {
		return runtime.NilValue(), err
	}
	if len(out) == 0 {
		return runtime.NilValue(), nil
	}
	return out[0], nil
}
