print("case:calls_multireturn")

assert(type(1 < 2) == "boolean")
assert(type(nil) == "nil")
assert(type(-3) == "number")
assert(type("x") == "string")
assert(type({}) == "table")
assert(type(type) == "function")

func pack4(a, b, c) {
    d := "sentinel"
    return a, b, c, d
}

a, b, c, d := pack4(1, 2)
assert(a == 1 && b == 2 && c == nil && d == "sentinel")

a, b, c, d = pack4(1, 2, 3, 4)
assert(a == 1 && b == 2 && c == 3 && d == "sentinel")

func tail(x) {
    if x <= 0 { return 101 }
    return tail(x - 1)
}
assert(tail(200) == 101)

func chain(x) {
    return x, x + 1, x + 2
}
x, y, z := chain(-2)
assert(x == -2 && y == -1 && z == 0)

print("ok")
