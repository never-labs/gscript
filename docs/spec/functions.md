# Functions

Functions are first-class callable values. They may be named declarations or
anonymous literals.

Parameters are lexical bindings initialized from call arguments. Missing
arguments become `nil`; extra arguments are discarded unless the function has a
vararg parameter.

Vararg parameters collect remaining arguments. Varargs participate in
multi-return adjustment and may be forwarded by call syntax.

Closures capture lexical variables by reference. Mutating a captured variable is
visible to all closures that share the binding.

Script functions and host functions share the same script-visible call result
model. Host functions may return structured recoverable errors where their
module contract specifies `nil, err` behavior.

Tail-call optimization is an implementation detail unless a section explicitly
promises stack behavior for a feature.
