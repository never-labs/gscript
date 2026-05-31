// Package methodjit implements the method JIT compiler pipeline.
//
// It owns Tier 1 and Tier 2 compilation from bytecode into native code:
// profiling and tiering policy, graph/SSA construction, analysis fact domains,
// optimization passes, register allocation, deoptimization exits, and emission
// orchestration. Low-level ARM64 encoding, executable memory management,
// runtime layout constants, and call trampolines are supplied by internal/jit.
package methodjit
