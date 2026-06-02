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

```leia run all
x := 1 + 2 * 3      // 7
y := (1 + 2) * 3    // 9
z := false || "ok"  // "ok"
w := nil && fail()  // nil; fail is not called

assert(x == 7)
assert(y == 9)
assert(z == "ok")
assert(w == nil)
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

Operator dispatch has three layers:

1. apply the primitive operation when the operands are already valid for it;
2. apply the operator's stable primitive coercions, if any;
3. otherwise consult the matching metamethod where metatables support that
   operator, then raise a runtime error if no applicable metamethod exists.

Numeric arithmetic operators accept numbers and strings that `tonumber` can
parse as numbers. Numeric string coercion is attempted before metamethod lookup
for primitive strings. Invalid numeric strings and unsupported operand types
raise runtime errors unless a matching metamethod applies to a non-primitive
operand. Library functions such as `tonumber`, `tostring`, `math.tointeger`,
and formatting helpers expose the same conversions explicitly.

| Operator | Operands and coercions | Result and errors | Metamethod |
|---|---|---|---|
| `+` | Numbers or numeric strings. | Addition. Exact integer operands may produce an integer; mixed or inexact results may be float. Invalid operands error. | `__add` |
| binary `-` | Numbers or numeric strings. | Subtraction with the same numeric representation rule as `+`. Invalid operands error. | `__sub` |
| `*` | Numbers or numeric strings. | Multiplication with the same numeric representation rule as `+`. Invalid operands error. | `__mul` |
| `/` | Numbers or numeric strings. | Division produces numeric runtime division semantics; integer inputs may produce a float. Invalid operands error. | `__div` |
| `%` | Numbers or numeric strings. | Modulo follows Leia numeric modulo semantics. Invalid operands error. | `__mod` |
| `**` | Numbers or numeric strings. | Exponentiation follows the math library's power semantics. Invalid operands error. | `__pow` |
| unary `-` | Number or numeric string. | Numeric negation. Invalid operands error. | `__unm` |
| `..` | Strings, numbers, and operands accepted by the runtime concatenation contract. | Concatenation returns a string for primitive operands. Invalid operands error. | `__concat` |
| `#` | String, table-like value, or host-backed value with length support. | Strings return byte length. Tables use ordinary sequence/metatable length behavior. Invalid operands error. | `__len`; `rawlen` bypasses it |
| `==` | Any values. | Primitive values compare by value. Tables, functions, channels, coroutines, and host values compare by identity unless stable equality metamethod dispatch applies. | `__eq` |
| `!=` | Any values. | Logical negation of `==`, including any stable `__eq` dispatch. | `__eq` |
| `<` | Compatible numbers or compatible strings. Numeric strings are not coerced for primitive string ordering; use `tonumber` explicitly. | Ordered comparison. Incompatible operands error. | `__lt` |
| `<=` | Compatible numbers or compatible strings. Numeric strings are not coerced for primitive string ordering. | Ordered comparison. Incompatible operands error. | `__le`; no fallback to `__lt` |
| `>` | Compatible numbers or compatible strings. Numeric strings are not coerced for primitive string ordering. | Equivalent to reversed `<` after dispatch. Incompatible operands error. | `__lt` with reversed operands |
| `>=` | Compatible numbers or compatible strings. Numeric strings are not coerced for primitive string ordering. | Equivalent to reversed `<=` after dispatch. Incompatible operands error. | `__le` with reversed operands; no fallback to `__lt` |
| `&&` | Any values. | Returns the left operand when it is falsy; otherwise evaluates and returns the right operand. | none |
| `||` | Any values. | Returns the left operand when it is truthy; otherwise evaluates and returns the right operand. | none |
| `!` | Any value. | Returns `true` for `nil` and `false`; returns `false` for every other value. | none |
| `&` | Numbers or numeric strings convertible to integers by runtime integer conversion. | Bitwise and. Invalid or non-integral operands error. | none |
| `|` | Numbers or numeric strings convertible to integers by runtime integer conversion. | Bitwise or. Invalid or non-integral operands error. | none |
| `^` | Numbers or numeric strings convertible to integers by runtime integer conversion. | Bitwise xor. Invalid or non-integral operands error. | none |
| `&^` | Numbers or numeric strings convertible to integers by runtime integer conversion. | Bit clear. Invalid or non-integral operands error. | none |
| `<<` | Integer-convertible left and shift count operands; numeric strings are accepted through the same conversion. | Left shift. Invalid operands error. | none |
| `>>` | Integer-convertible left and shift count operands; numeric strings are accepted through the same conversion. | Right shift. Invalid operands error. | none |
| `<-` | Channel value. | Receive blocks until a value is received, the channel is closed, or the host cancels execution. Invalid operands error. | none |

Raw helpers bypass their corresponding metamethods: for example, `rawget`,
`rawset`, `rawequal`, and `rawlen` use raw table/string behavior where they are
defined.

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
assert("a" .. 3 == "a3")
assert((1 << 8) == 256)
```

```leia fail all
return "x" + 3
```

```leia run all
boxed := setmetatable({value: 4}, {
    __add: func(left, right) {
        return left.value + right
    },
    __len: func(_) {
        return 9
    },
})

assert(boxed + 3 == 7)
assert(#boxed == 9)
assert(rawlen(boxed) == 0)
```

## Literals

Table literals, list literals, dense array literals, function literals,
anonymous agent expressions, `turn` expressions, and `messages` blocks are
expressions. Their specific syntax is listed in [grammar.ebnf](grammar.ebnf).

Literal operands are evaluated left-to-right. A literal that constructs an
identity-bearing value creates a fresh identity each time the literal is
evaluated. This applies to table literals, function literals, anonymous agent
expressions, `turn` request objects, `messages` arrays, and dense arrays.

Table fields are evaluated in source order. For stable v1.0 programs, avoid
depending on duplicate keys or on subtle interleaving between list-style and
keyed fields; [Tables And Metatables](tables.md) defines the portable
constructor subset. List literals evaluate each element in order and store the
results as a new 1-based array table. Dense array literals evaluate each
element in order, convert each element according to the dense element type, and
raise a runtime error if a value cannot be represented by that element type.
Function and agent literals capture their lexical environment by reference.
`messages` blocks evaluate role fields in order and create a new ordered
message array.

```leia run all
events := {}
func mark(name, value) {
    events[#events + 1] = name
    return value
}

t := {
    first: mark("field", 1),
    mark("list", 2),
}
list := [mark("a", 10), mark("b", 20)]
dense := []i64{mark("d1", 3), mark("d2", 4)}

assert(events[1] == "field")
assert(events[2] == "list")
assert(events[3] == "a")
assert(events[4] == "b")
assert(events[5] == "d1")
assert(events[6] == "d2")
assert(t.first == 1)
assert(t[1] == 2)
assert(list[1] == 10 && list[2] == 20)
assert(dense[1] == 3 && dense[2] == 4)
assert({} != {})
```

## Evaluation Order

Within an expression list, subexpressions are evaluated left-to-right unless a
specific expression form short-circuits. Implementations may optimize execution
but must preserve observable side effects and error behavior.

```leia run all
events := {}
func mark(name) {
    events[#events + 1] = name
    return name
}

result := mark("left") .. mark("right")
assert(result == "leftright")
assert(events[1] == "left")
assert(events[2] == "right")

func fail_if_called() {
    error("should not be called")
}

short_and := false && fail_if_called()
short_or := true || fail_if_called()
assert(short_and == false)
assert(short_or == true)
```
