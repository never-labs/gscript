# GScript

Dynamically-typed scripting language with Go syntax and Lua semantics. Three-tier execution on Apple Silicon ARM64: **interpreter → Tier 1 baseline JIT → Tier 2 optimizing JIT**.

Tier 2 IR pipeline: `BuildGraph → TypeSpec → Intrinsic → Inline → ConstProp → LoadElim → DCE → RangeAnalysis → LICM → RegAlloc → ARM64`.

## Tier 2 Optimizer Architecture

The Tier 2 optimizer uses a **module registry** pattern. Each optimization domain (frontend, table, numeric, string) registers its phase-specific module builders via `init()` functions. The central `BuildModulePlan()` assembles the final ordered plan from all registered builders.

### Key files (`internal/methodjit/`)

- `tier2_optimizer_modules.go` — Phase constants, `Tier2OptimizerModule` type, plan execution engine
- `modules_registry.go` — `RegisterModuleBuilder()`, `BuildModulePlan()`, `ModuleBuilder` type
- `dependency_check.go` — `ValidateDependencyOrder()` for fact-based dependency validation
- `analysis_result.go` — `AnalysisResult` struct: all analysis maps produced/consumed by passes
- `pipeline.go` — `RunTier2Pipeline()` entry point, `Tier2PipelineOpts`, `PassFunc` type

### Domain-specific module files

- `tier2_optimizer_modules_frontend.go` — early_canonical, inline_call, call_lower, post_rewrite, final_call phases
- `tier2_optimizer_modules_table.go` — table_object_prep, table_array_lower, table_field_lower phases
- `tier2_optimizer_modules_numeric.go` — numeric, matrix_native, float_numeric, loop_kernel, loop_post phases
- `tier2_optimizer_modules_string.go` — string_native phase

### Adding a new optimization pass

1. Write the pass function (signature `func(*Function) (*Function, error)`)
2. Register it in the appropriate domain file's builder function using `tier2PassModuleWith()`
3. If it needs a new phase, add a `Tier2Phase*` constant and register a builder with `RegisterModuleBuilder()` in an `init()` function

## Tools

```bash
# Full bench suite (VM / JIT / LuaJIT)
bash benchmarks/run_all.sh [--runs=N]

# Statistical regression guard with checksum + CV; covers suite + extended + variants
python3 benchmarks/strict_guard.py [--bench <suite>/<name>] [--runs N]

# Current vs HEAD vs LuaJIT timing comparison
python3 benchmarks/timing_compare.py --all-groups [--runs N] [--sort=luajit-gap]

# Production-parity Tier 2 IR/asm dump
bash scripts/diag.sh <bench>            # bare name searched suite→extended→variants
bash scripts/diag.sh <suite>/<bench>    # explicit
# Output: diag/<suite>/<bench>/{<proto>.bin,.asm.txt,.ir.txt,stats.json}

# In-process oracle: IR-interpreter vs ARM64 native, full pass log
Diagnose(proto, args)
```

JIT native code is opaque to `pprof`; use `Diagnose()` and the ARM64 disasm under `diag/` for Tier 2 inspection. `pprof` is valid for Go-runtime / interpreter paths.
