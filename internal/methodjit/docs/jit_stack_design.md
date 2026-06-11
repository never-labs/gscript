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
