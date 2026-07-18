// Package leia provides the public Go API for embedding the Leia scripting
// language.
//
// A VM executes Leia source and compiled programs, exposes explicit Go-backed
// functions and modules, and can be configured with library, capability,
// sandbox, and resource-budget options. VM instances are mutable and are not
// safe for concurrent use; concurrent hosts should use separate VMs or Pool.
//
// Hosts that execute untrusted scripts should begin with SecuritySandbox and
// grant back only the capabilities and bindings their application requires.
package leia
