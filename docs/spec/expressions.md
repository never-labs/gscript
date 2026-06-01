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

```leia
x := 1 + 2 * 3      // 7
y := (1 + 2) * 3    // 9
z := false || "ok"  // "ok"
w := nil && fail()  // nil; fail is not called
```

## Calls

A call expression invokes a callable value. Calls may produce zero or more
results. A call used where exactly one expression value is required contributes
its first result, or `nil` when it produces no results. Expression-list
positions are adjusted by the multi-return rules in [Functions](functions.md).

```leia
func pair() {
    return "a", "b"
}

x, y := pair()          // x == "a", y == "b"
first := pair()         // first == "a"
```

The built-in `spread(x)` form preserves multiple results from a call in
argument and table-constructor positions where explicit expansion is required.

```leia
values := {1, spread(pair()), 4}
// values is {1, "a", "b", 4}
```

## Indexing And Member Selection

`x[y]` indexes a table-like or host-backed value. `x.name` is member selection
and is equivalent to a string-key field lookup where supported.

```leia
user := { name: "Ada" }
same := user.name == user["name"]
user["score"] = 10
```

Metamethods may affect indexing, assignment, calls, arithmetic, comparison, and
length behavior as specified in [Tables And Metatables](tables.md).

## Operators

Numeric operators accept numbers and strings that `tonumber` can parse as
numbers. Invalid numeric strings and unsupported operand types raise runtime
errors unless a matching metamethod applies. Library functions such as
`tonumber`, `tostring`, `math.tointeger`, and formatting helpers expose the
same conversions explicitly.

| Operator family | Stable behavior |
|---|---|
| `+ - * / % **` | Numeric arithmetic. Numeric strings are converted before the operation. Integer operands may keep integer representation when the operation is exact; otherwise results may be floating-point. Unsupported operands raise a runtime error unless a matching arithmetic metamethod applies. |
| unary `-` | Numeric negation or `__unm` on values with a metatable handler. |
| `..` | String concatenation. Non-string primitive operands may be converted using `tostring` according to the runtime string contract; table-like operands may use `__concat`. |
| `#` | Length operation. Strings return byte length. Tables use ordinary sequence/metatable length behavior; `rawlen` bypasses `__len`. |
| `== !=` | Equality and inequality. Primitive values compare by value; tables, functions, channels, coroutines, and host values compare by identity unless a stable equality metamethod applies. |
| `< <= > >=` | Ordered comparison over compatible primitive values or comparison metamethods. Incompatible values raise a runtime error. |
| `&& || !` | Logical operations using Leia truthiness. `&&` and `||` short-circuit and return operand values. |
| `& | ^ &^ << >>` | Integer bitwise operations. Operands must be numbers or numeric strings that can be converted to integers according to the runtime integer conversion rules; invalid operands raise a runtime error. |
| `<-` | Channel receive in expression position. It blocks until a value is received, the channel is closed, or the host cancels execution. |

```leia
1 + 2        // 3
"5" + 3      // 8
"x" + 3      // runtime error
"a" .. 3     // "a3"
(1 << 8)     // 256
```

```leia run
assert(1 + 2 == 3)
assert("5" + 3 == 8)
ok, _ := pcall(func() { return "x" + 3 })
assert(!ok)
assert("a" .. 3 == "a3")
assert((1 << 8) == 256)
```

## Literals

Table literals, list literals, dense array literals, function literals,
anonymous agent expressions, `turn` expressions, and `messages` blocks are
expressions. Their specific syntax is listed in [grammar.ebnf](grammar.ebnf).

## Evaluation Order

Within an expression list, subexpressions are evaluated left-to-right unless a
specific expression form short-circuits. Implementations may optimize execution
but must preserve observable side effects and error behavior.

```leia
events := {}
func mark(name) {
    events[#events + 1] = name
    return name
}

result := mark("left") .. mark("right")
// events is {"left", "right"}
```
