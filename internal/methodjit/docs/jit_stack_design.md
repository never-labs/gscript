# Dedicated non-Go stack for Tier 2 JIT execution (R5-K)

Status: **working prototype, flag-gated** (`LEIA_JIT_ALT_STACK=1`).
Verdict: **GO on the mechanism, NO-GO (for now) on enabling it by default** —
see "Measurements" and "Recommendation".

## Problem

Tier 2 native code runs on the goroutine stack via the NOSPLIT trampoline
`jit.CallJIT` (`internal/jit/trampoline_arm64.s`), with the stack pre-grown by
`tier2_stack.go`. JIT frames carry no funcdata/stack maps, so the Go runtime
can neither precisely unwind nor copy a stack that has JIT frames in the
middle of it. The runtime tolerates this today only because a goroutine inside
JIT code is **never at an async-preemption-safe point**: `suspendG`/GC stack
scans simply wait until the code returns to Go.

Consequence (R5-J finding): JIT code cannot BLR into a preemptible Go helper.
While the helper ran, the runtime *could* suspend the goroutine, and
`gentraceback` would hit the JIT frame ("unknown pc" fatal in GC scans) or
`copystack` would silently corrupt it. Every JIT→Go interaction therefore
bounces through the exit protocol — full unwind out of native frames, Go-side
dispatch, re-entry at a resume offset with a fresh prologue — even on the slim
lane (`tiering_exit_fast_q_eval.go`).

## Design (implemented)

JIT frames move to a dedicated mmap'd stack (`internal/jit/jitstack.go`,
`jit.JITStack`: guard page at the low end, 192-byte transition header at the
top). Three asm pieces in `internal/jit/trampoline_stack_arm64.s`:

### 1. `callJITOnStack(fn, ctx, hdr)` — entry trampoline

NOSPLIT|NOFRAME **leaf with no goroutine-stack frame at all**. It saves the
goroutine SP/FP/LR and the g pointer into the JIT stack header, sets
`SP = hdr`, and CALLs the JIT code. On return it restores everything from the
header. Because it keeps no frame, the unwind chain during a helper call can
skip it entirely: the fabricated frame below claims `callJITOnStack`'s Go
caller as its own caller. Its SPWRITE flag is harmless — its (nonexistent)
frame never appears mid-stack in a precise traceback.

### 2. `jitGoHelperEntry` / `jitGoHelperBody` / `jitGoHelperReturn` — direct helper call

JIT code BLRs `jitGoHelperEntry` with `R0 = hdr`, `R19 = ctx`:

- **Entry** (NOFRAME, innermost-only SPWRITE): saves the full JIT register
  state (X19–X28, D8–D11, FP, SP, resume LR) into the header, restores the Go
  environment (`g`, goroutine SP/FP from the header) and **fabricates
  `R30 = hdr.goLR`** — the return PC into the Go caller of `callJITOnStack` —
  then branches to the body. This is the same technique `runtime.cgocallback`
  uses when it pushes the saved gobuf PC so the traceback "seamlessly traces
  back into the earlier calls".
- **Body** (NOSPLIT|NOFRAME, hand-built 48-byte compiler-style frame: saved LR
  at `0(SP)`, caller FP at `-8(SP)`, `R29 = SP-8`): the one real goroutine-
  stack frame that exists while the helper runs. It calls the registered Go
  bridge (`jit.SetJITHelperBridge` → `methodjit.tier2JITHelperBridge`). While
  the helper runs, the runtime sees a fully legal chain
  `helper → jitGoHelperBody → (Go caller of callJITOnStack) → …` and every
  runtime facility works: precise GC stack scans, stack growth **and** shrink
  (copystack), async preemption, panic unwinding, profiling.
- **Return** (NOFRAME, innermost-only SPWRITE): after the body pops its frame,
  refreshes `hdr.goSP`/`hdr.goFP` from the live registers — **this is the
  copystack fix-up**: if the goroutine stack moved while the helper ran, the
  header copies are stale and the refreshed values are the only correct ones —
  then restores the JIT register state and RETs back into JIT code.

The final return in `callJITOnStack` also reads SP/FP from the header, so a
copystack during any helper call is transparently absorbed.

### Two assembler landmines (cost a debugging cycle each)

- **pcsp**: in hand-written arm64 asm, only `ADD/SUB $const, RSP` get `Spadj`
  (and hence pcsp entries). Pre/post-indexed writeback forms (`MOVD.W R30,
  -48(RSP)` / `MOVD.P`) leave the pcsp table flat — the unwinder then walks
  with the wrong frame size, reads return PCs from the wrong slots, and
  copystack "adjusts" garbage ("bad pointer in frame …"). The body's frame
  must be opened/closed with tracked forms only.
- **frame-size rounding**: a framed `TEXT $40-0` gets a 64-byte assembler
  frame on arm64, not 48; pairing an assembler prologue with a manual epilogue
  desyncs SP by 16. Hence NOFRAME + fully hand-built frame.

Also: any instruction with `To.Reg == REGSP` and no Spadj marks the whole
function SPWRITE; the body must therefore never write SP from a register
(traceback refuses to unwind *past* a mid-stack SPWRITE frame; entry/return
may be SPWRITE because they only ever execute as the innermost frame).

## Answers to the study questions

**(a) Does the runtime ever need to unwind across the JIT stack?** No, by
construction. While SP is on the JIT stack the goroutine is never at a safe
point (same invariant as today). While a helper runs, the goroutine-stack
chain is complete and legal *without* any JIT frame in it; the JIT stack is
parked, referenced only by the header. The two stacks never appear in one
unwind.

**(b) What must g.stack/g.sched look like?** Untouched. `g.stack` still
describes the goroutine stack, which stays valid and scannable (the
`executeTier2` frame keeps `ctx`, `cf`, and the register file alive — emitted
direct-call sites must keep all live boxed values spilled to the heap register
file across the call, which the existing exit-spill discipline already does).
`g.sched` is only written by the runtime at real safe points, which only occur
inside helpers, where SP/FP are genuine goroutine-stack values. The only
SP-shaped state we own — the header's goSP/goFP — is refreshed after every
helper return. The g pointer is stashed in the header at entry (`g` never
changes for a goroutine's lifetime, so helper-time M migration is fine);
required because Tier 2 register allocation uses X28 (the g register) as a
general-purpose register.

**(c) Panics / signals inside helpers?** A helper panic unwinds the fabricated
chain like a normal panic — deferred functions in `executeTier2` and its
callers run, the JIT stack is abandoned mid-frame and simply re-armed on next
acquisition from the pool. (Resuming JIT code after a recovered helper panic
is *not* supported — same contract as an error return ending the execution.)
Signals: profiling signals while in JIT code fail the traceback gracefully
exactly as today (unknown PC); async preemption is never injected at JIT PCs
or inside the asm thunks (asm is never async-safe), and inside helpers it is
plain Go. Notably the helper calls *give the scheduler its preemption points
back*: a `stackPreempt`-poisoned stackguard fires on the helper's first
prologue check, on a valid goroutine stack.

## Wiring (methodjit)

- `tier2_alt_stack.go`: `sync.Pool` of 512 KiB stacks; bridge dispatcher;
  `Tier2DirectHelperCallCount()` engagement counter. Nested Tier 2 executions
  started from inside a helper acquire their *own* stack — the only hard rule
  is that a helper must never resume the suspended JIT stack itself.
- `tiering_execute.go` (`executeTier2`) and the native-callee resume loop in
  `tiering_manager_exit_native.go` (`resumeNativeTier2CalleeExit`) both run
  native code via `jit.CallJITOnStack` when enabled and publish
  `ctx.JITStackHdr`/`ctx.HelperCF`. Legacy `jit.CallJIT` paths (Tier 1,
  Diagnose, emit_execute standalone) leave `ctx.JITStackHdr = 0`.
- `emit_q_eval_session_eval.go`: the OpQEvalSessionEval site loads
  `ctx.JITStackHdr`, **CBZ-falls back to the generic op-exit when 0** (so one
  compiled image is correct on both trampolines), otherwise BLRs the thunk;
  on `ctx.HelperErrFlag` it exits with the new `ExitQEvalHelperErr` (error in
  `ctx.HelperErr`, identical "tier2: op-exit: %w" wrap, no re-execution).

## Verification

All on Apple Silicon (M4 Max), Go 1.25.7:

- `go test ./internal/methodjit -count=1` green in both modes; benchmarks
  package green in both modes; `TestQEvalJITScriptRouting` green in alt mode.
- `internal/jit` stress (`TestJITGoHelperCallUnderGC`,
  `TestJITGoHelperCallPreemption`): helpers force `growStack(512)` (copystack
  with the fabricated frame mid-stack) every 16th call and `runtime.GC()`
  (precise scan) every 1024th, with a sibling GC-hammer goroutine, 4
  concurrent JIT goroutines — green, repeatedly, including under
  `GOGC=1 GODEBUG=gcstoptheworld=0,asyncpreemptoff=0`.
- Production engagement: `TestJITAltStackDirectHelperEngagement` — 256 direct
  helper calls over 256 q session-eval loop iterations (1.000 direct/eval),
  results identical to the Go baseline; also green under `GOGC=1`.
- `runtime.Callers` from inside a production helper resolves the full
  fabricated chain: `executeQEvalSessionEval → tier2JITHelperBridge →
  jit.helperBridge → jit.jitGoHelperBody → jit.CallJITOnStack →
  executeTier2WithResultBuffer → … → testing.tRunner → runtime.goexit`.
- No "unknown pc", no SPWRITE throws, no stack corruption observed in any
  configuration after the pcsp fix.

## Measurements

Transition micro-costs (`internal/jit` benches):

| round trip                                   | ns    |
|----------------------------------------------|-------|
| `CallJIT` (legacy, goroutine stack)          | ~1.3  |
| `CallJITOnStack` (alt stack)                 | ~2.2  |
| direct BLR helper call (thunk+bridge, noop)  | ~3.9  |

`BenchmarkQEvalJITSessionEvalLane` (per-iteration q session eval, warmed and
measured with the same argument so the loop stays in Tier 2; 512 evals/call,
5×2s runs):

| mode                          | ns/eval        | direct/eval |
|-------------------------------|----------------|-------------|
| default (slim exit lane)      | 305.2–308.4    | 0           |
| `LEIA_JIT_ALT_STACK=1`        | 303.9–307.1    | 1.000       |

**The direct lane fully engages and is correct, but is performance-neutral
here.** The eliminated round trip (epilogue + Go dispatch + resume prologue
on the already-optimized slim lane) is ~10 ns against a ~305 ns helper whose
cost is the typed q kernels themselves (~4 allocs/eval). The ~4 ns direct-call
transition eats most of the theoretical saving.

Caveat discovered while measuring: `BenchmarkQEvalJITScriptWarm` warms with
`run(4)` and measures `run(b.N)`; the first post-warmup call takes an
argument-shape deopt (`deoptID=25`, entry guard) and the remainder of that
call runs outside Tier 2. The sub-2µs band numbers therefore do not measure
the per-iteration exit lane today; `BenchmarkQEvalJITSessionEvalLane` was
added for a controlled comparison. (Pre-existing on main, both modes.)

## Recommendation

- **Mechanism: GO.** The cgocallback-style fabricated-frame discipline works
  against the unmodified Go 1.25 runtime and survives every stress we could
  throw at it (precise scans, copystack growth *and* shrink, async
  preemption, multi-goroutine, GOGC=1). The two failure modes hit during
  development were both self-inflicted asm-metadata bugs (pcsp tracking,
  frame-size rounding), not runtime blockers, and both are documented above.
  Maintenance risk: the discipline depends on stable runtime behaviors
  (LR-at-`0(SP)` arm64 frame layout, `adjustframe`'s FP-slot rule, traceback's
  SPWRITE policy) that are de-facto frozen by `cgocallback` using the same
  tricks, but it should be re-validated per Go release (the stress tests do
  this automatically).
- **Default-on for the q session-eval site: NO-GO for now.** It is
  performance-neutral on the workload that motivated it because the slim exit
  lane already amortized the Go-side overhead and the helper dominates.
  Keep it flag-gated.
- **Where the prize actually is**: helpers in the tens-of-ns class called
  many times per iteration — per-element typed kernels, string interning,
  allocation-free runtime probes — where a 1.3 ns→3.9 ns call beats a
  10–30 ns exit round trip *and* removes the resume-entry/spill-pressure tax
  on emitted code (no resume entries, no deferredResumes, callee-saved state
  preserved across the call so selective spill sets shrink). The next
  experiment should target one of those, not another coarse-grained op-exit.

## R6-S follow-up: candidate ranking on real workloads + second conversion

This section reports the R6-S application of the prototype to real workloads:
a data-driven ranking of every JIT→Go transition on the benchmark corpus, the
conversion of the #2 transition (`ExitQEvalPipelinePlan`) to the direct lane,
a correctness fix the conversion surfaced in the prototype itself, and the
measured A/B. All on Apple M4 Max, Go 1.25.7, same binary, flag A/B.

### Transition ranking (frequency × per-call overhead share)

Measured with `leia bench profile-exits` (CLI `-exit-stats-json`, default
and `LEIA_TIER2_NO_FILTER=1` modes, all domains), plus a forced-Tier 2 sweep
of the full q JIT script family (all 482 `qEvalVectorCases` × 64 iterations,
aggregating `ExitStats` + `QKernelExecutionStats`):

| # | transition | observed frequency | helper cost | exit share | action |
|---|------------|--------------------|-------------|-----------|--------|
| 1 | `OpQEvalSessionEval` (slim `ExitOpExit` lane) | exactly 1.000/iteration across all 482 q script cases (30,848 execs / 30,848 iters); the ONLY per-iteration transition in the family | ~305 ns – 6 µs (typed q kernels) | ~10 ns slim lane ⇒ ≤3 % | already converted (R5-K) |
| 2 | `OpQEvalPipelinePlan` (`ExitQEvalPipelinePlan`) | 1/iteration when the plan is EXECUTABLE (direct `q.eval(const)` loops; pinned by `TestJITAltStackDirectPipelinePlanEngagement`: 64 exits per `run(64)`); 1/Tier 2 entry when heuristic (hoisted/memoized) | ~2.1 µs (`+/til 64`) | ~100–150 ns generic protocol incl. the global exit-stat mutex ⇒ ~5 % | **converted here** |
| 3 | `ExitCallExit` (generic `Call`) | 1 per `run()` call in the q family (session setup); 2–3 per whole run elsewhere | µs-scale (full VM call) | <1 % | no |
| 4 | `ExitTableExit` (SetTable/GetField/NewTable) | zero in default mode; only visible under `LEIA_TIER2_NO_FILTER=1` (worst: `table/groupby_nested_agg`, 56 exits in a 0.39 s run) and these are one-shot learning exits that mature recompile feedback | n/a | n/a | no — converting would starve `recordTier2ExitProfile` feedback |
| 5 | Frame/Vector typed runtime routes (`OpFrame*`, `OpVector*`, `OpQFrameSelectColumn`, `OpQVectorWhereReduce`, …) | **zero** exits across every `.leia` suite (default + no-filter, incl. all `data/q_*` and `soa_*` rows) and the q script family | µs bulk kernels | n/a | no — no frequency, bulk-amortized |
| 6 | q array bridge (`qEvalPipelineArrayRuntimeValue`, `BenchmarkQEvalPipelineArrayRuntimeBridge`) | not a JIT→Go transition at all — it is a Go→Go conversion inside pipeline helpers (1.2 µs / 8192 rows, bulk) | — | — | not a candidate |
| 7 | string/table runtime helpers (`Concat`, `StringFormat*`, …) | 1–4 exits per whole benchmark run | — | — | no |

The structural finding: after the R6 native-lowering work the optimizer has
already eliminated per-iteration exits from every non-q domain (whole
benchmark runs show single-digit TOTAL exits, all one-shot
deopt/feedback events). The q runtime helpers are the only per-iteration
JIT→Go transitions left, and both are in the hundreds-of-ns-to-µs class. The
tens-of-ns × many-per-iteration helper population the direct lane was built
for **does not currently exist** on real workloads.

### Conversion 2: `ExitQEvalPipelinePlan` → direct BLR (this change)

Same bridge pattern as the session-eval site, flag-gated, one image correct
in both modes (CBZ on `ctx.JITStackHdr`):

- `emit_q_eval_pipeline.go`: gated direct block. The dedicated exit protocol
  never staged `OpExitOp`, so the direct block stages it for bridge dispatch
  (the generic fallback path ignores it). X0 must hold the stack header at
  the BLR (thunk ABI), so all descriptor staging happens before the header
  load.
- `tier2_alt_stack.go`: bridge case runs the EXACT generic handler
  (`executeQEvalPipelinePlanExit`, route `typed_runtime_native_exit`) against
  the live register file reconstructed from `ctx.RegsBase/RegsEnd/Regs`
  (`helperRegsWindow`); the route name, per-plan counters, and error object
  are identical to the generic path.
- **Exit-stat parity**: unlike the session-eval op (which has its own
  lock-free counters and whose slim lane already skipped `ExitStats`), the
  generic pipeline exit records an `ExitStats` row per exit, and routing
  tests pin that signal. The bridge therefore records the same row via
  `ctx.HelperTM` (new Go-only ExecContext field) before executing — direct
  mode is diagnostically indistinguishable from the generic protocol. This
  knowingly retains the global mutex; see "Measurements".
- **Error semantics**: helper errors exit through `ExitQEvalHelperErr`; the
  Go loops select the wrap by `ctx.OpExitOp` — pipeline errors keep the
  exact generic wraps (`"tier2: q eval pipeline exit: %w"` /
  `"callee q eval pipeline exit: %w"`), session/op errors keep
  `"tier2: op-exit: %w"` / `"callee op-exit: %w"`.
- **Terminal-return plans** need no special casing: the generic path's early
  return is an optimization; the direct lane simply continues into the
  function's own return sequence and produces the identical result and
  feedback merge through `ExitNormal`.
- **resyncRegs** is not replicated: like the session-eval executors, the q
  pipeline backends are host Go over q runtime state and never re-enter the
  Leia VM, so the register file cannot move while they run.

### Prototype bug found and fixed: callee-cf confusion (self-cf cells)

`ctx.HelperCF` was published once per native execution with the ENTRY
function's cf. But the ExecContext is shared across the whole native
execution, including natively-BLR'd Tier 2 callees — a direct site inside a
callee would have dispatched against the CALLER's CompiledFunction (wrong
plan tables, wrong constant pool: silently wrong eval source in the worst
case). The session-eval prototype site had this latent bug; the pipeline op
made it reachable in practice (callee pipeline exits are an established path,
see `resumeNativeTier2CalleeExit`).

Fix: each Tier 2 compilation pre-allocates a **self-cf cell** (`*uintptr`,
`emit_compile.go`), embeds the cell's address in every direct site, and fills
the cell with the CompiledFunction pointer after construction. Every direct
site re-publishes its OWN cf to `ctx.HelperCF` (one load + one indirection +
one store) immediately before the BLR. The native call sequence keeps
`ctx.Regs` callee-correct already (stored before/after the BLR), so the
bridge's register window is consistent with the cf. Pinned by
`TestJITAltStackDirectHelperCalleeCF` (natively-called `work()` containing
`q.eval(const)`, exercised in both modes).

### Measurements (R6-S)

Same-binary interleaved flag A/B, 6 rounds × 500 calls, 512 evals/call
(`benchmarks/jit_alt_stack_direct_test.go`):

| bench (ns/eval) | default (exit protocol) | `LEIA_JIT_ALT_STACK=1` (direct) | direct/eval |
|---|---|---|---|
| `BenchmarkQEvalJITSessionEvalLane` | 313.6–323.3 (mean 317.7) | 312.3–318.5 (mean 315.5) | 1.000 |
| `BenchmarkQEvalJITPipelinePlanLane` | 2112–2157 (mean 2133) | 2112–2207 (mean 2142) | 1.000 |

Session eval: alt ≤ default in 5/6 paired rounds (≈ −0.7 %, at the edge of
noise). Pipeline plan: paired deltas −23…+51 ns on a 2.1 µs helper —
**neutral within noise**; the ~100 ns protocol saving is partly returned by
the retained exit-stat recording (parity requirement) and the ~4 ns direct
transition, and the remainder is unresolvable against a 2.1 µs helper.

Real rows (CLI, 5 reps/mode, same binary): `q_operator_pipeline`
0.435/0.434 s, `q_query_rollup` 0.398/0.398 s, `frame_qsql_rollup`
0.296/0.297 s, `soa_masked_aggregate` 0.186/0.188 s (off/on) — flat, as the
ranking predicts (these rows have zero Tier 2 exits).

Stress: `TestJITGoHelperCall*` under
`GOGC=1 GODEBUG=gcstoptheworld=0,asyncpreemptoff=0` and the full
`TestJITAltStackDirect*` family under `GOGC=1` ×3 — green in both modes.
Full `internal/jit`, `internal/methodjit`, `internal/stdlib/...`,
`internal/vm`, `benchmarks` suites green in both modes, including
`TestQEvalJITScriptRouting` and `TestDiag_ProductionParity_*`.

### R6-S verdict

- **Conversions**: `OpQEvalSessionEval` (R5-K) and `OpQEvalPipelinePlan`
  (R6-S) both run on the direct lane under the flag, fully engaged
  (1.000 direct/eval), semantics-exact (results, routes, error wraps,
  exit-stat diagnostics), and now callee-safe via self-cf cells.
- **Default-on: NO-GO, reaffirmed with corpus-wide data.** Both converted
  helpers are dominated by their own kernel cost; every other transition on
  real workloads is either ~zero-frequency, bulk-amortized, or a feedback
  mechanism that must keep exiting. There is no measured win to buy with the
  added mode, and the bar (consistent wins on real rows) is not met.
- **Keep the flag and the tests.** The mechanism stays validated per Go
  release by the stress suite. Re-evaluate when a genuinely fine-grained
  per-element helper route lands (e.g. per-row q kernels instead of bulk
  columnar kernels, string interning, allocation probes) — that is a
  prerequisite, not a tuning matter: the ranking shows the helper population
  the lane was designed for has been optimized out of existence everywhere
  else.
