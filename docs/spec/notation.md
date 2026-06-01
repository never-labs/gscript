# Notation

Leia syntax is specified with Extended Backus-Naur Form.

```ebnf
Production  = production_name "=" Expression "." ;
Expression  = Term { "|" Term } ;
Term        = Factor { Factor } ;
Factor      = production_name | token | Group | Option | Repetition ;
Group       = "(" Expression ")" ;
Option      = "[" Expression "]" ;
Repetition  = "{" Expression "}" ;
```

The grammar appendix uses semicolons at the end of productions:

```ebnf
program = { separator | statement } EOF ;
```

Lowercase production names denote lexical tokens or helper productions.
Double-quoted strings denote literal tokens. Informal prose may use ellipses
for omitted examples; an ellipsis is not a source token unless written as the
three-character token `...`.

The words must, must not, may, implementation-defined, stable, and
experimental have their ordinary specification meaning:

- must and must not describe required behavior;
- may describes permitted behavior;
- implementation-defined behavior must be documented by an implementation when
  exposed to users;
- stable behavior is part of the release compatibility contract;
- experimental behavior may change without compatibility guarantees.

Examples are explanatory unless a surrounding section says they are normative.
