# Internals

This section is for short implementation notes that are useful to maintainers
but not part of the public user contract.

New internals documents should be short and tied to stable package boundaries:

- parser and AST;
- runtime and value model;
- bytecode VM;
- method JIT and optimizer pipeline;
- stdlib `lib` / `bind` / `install` layering;
- package manager and module resolver.
