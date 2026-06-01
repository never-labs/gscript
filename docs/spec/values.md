# Values And Types

Leia is dynamically typed. Types are properties of runtime values, not declared
static variable slots.

Stable value categories are:

- nil;
- booleans;
- numbers;
- strings;
- tables;
- functions;
- coroutines;
- channels;
- host-backed values represented through tables or native functions.

Only `nil` and `false` are falsy. Numbers, including `0`, empty strings, empty
tables, functions, coroutines, and channels are truthy.

Numbers are represented as integers or floating-point values where possible.
Arithmetic may preserve integer representation when exact and fall back to
floating-point or boxed runtime operations. JIT raw integer representations are
not observable.

Strings are immutable byte strings. Library functions may interpret strings as
UTF-8, paths, JSON, or protocol data when their module contract says so.

Tables are mutable identity-bearing key/value objects. Arrays, records, dense
vectors, matrices, and SOA layouts are optimized representations or standard
library structures unless a future spec promotes them to primitive value
categories.

Functions are callable values. Host functions and script functions share call
semantics but may differ in performance, resource accounting, and recoverable
host error behavior.
