// Package jit provides the low-level native-code substrate used by the VM and
// higher-level JIT compilers.
//
// It owns ARM64 instruction encoding, executable memory allocation, direct-call
// trampolines, and ABI/layout constants that native code must share with the
// runtime. It should stay free of compiler pipeline policy: IR construction,
// tiering decisions, optimization passes, register allocation strategy, and
// language-level specialization belong in packages such as internal/methodjit.
package jit
