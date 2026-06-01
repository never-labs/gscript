# Implementation Requirements

The interpreter is the semantic baseline. Bytecode VM execution and JIT
execution must preserve interpreter-visible behavior for stable language
features.

An optimization may specialize at runtime after guards, but it must not depend
on benchmark names, source file names, or fixed benchmark input sizes. Guard
failure must deoptimize or recover without changing user-visible results.

Implementation details are not observable language behavior:

- table layout;
- dense numeric representation;
- raw integer JIT representation;
- inline caches;
- native call ABI;
- garbage collection timing;
- scheduling details except for specified channel and synchronization behavior.

Release gates must connect stable features to tests through
`tests/feature_matrix.json`. Changes to stable syntax require parser tests,
grammar updates, and conformance or semantic tests. Changes to stable runtime
behavior require VM/interpreter coverage and, when performance-sensitive,
benchmark coverage.
