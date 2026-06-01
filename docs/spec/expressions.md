# Expressions

Expressions compute values. Evaluation proceeds according to the expression
form and may raise runtime errors unless protected by a recovery construct.

## Precedence

Operators follow this precedence, from highest to lowest:

1. postfix call, index, and member selection;
2. unary operators;
3. exponentiation;
4. multiplicative arithmetic;
5. additive arithmetic and concatenation;
6. shifts and bitwise operators;
7. comparisons;
8. logical `&&`;
9. logical `||`;
10. assignment forms.

Parentheses override precedence. `&&` and `||` short-circuit and return operand
values rather than coerced booleans. Unary logical negation is `!`.

## Calls

A call expression invokes a callable value. Calls may return multiple values.
In the final position of an expression list, a call may expand to multiple
values. In non-final positions, it contributes one value unless a syntax form
explicitly spreads values.

## Indexing And Member Selection

`x[y]` indexes a table-like or host-backed value. `x.name` is member selection
and is equivalent to a string-key field lookup where supported.

Metamethods may affect indexing, assignment, calls, arithmetic, comparison, and
length behavior as specified in [Tables And Metatables](tables.md).

## Literals

Table literals, list literals, dense array literals, function literals,
anonymous agent expressions, `turn` expressions, and `messages` blocks are
expressions. Their specific syntax is listed in [grammar.ebnf](grammar.ebnf).

## Evaluation Order

Within an expression list, subexpressions are evaluated left-to-right unless a
specific expression form short-circuits. Implementations may optimize execution
but must preserve observable side effects and error behavior.
