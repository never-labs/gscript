print("case:vararg_pack")

func count(...) {
    return select("#", ...)
}

func collect(a, b, ...) {
    rest := table.pack(...)
    return a, b, rest.n, rest[1], rest[2]
}

assert(count() == 0)
assert(count(10, 20, 30) == 3)

a, b, n, x, y := collect(1, 2, 3, nil, 5)
assert(a == 1 && b == 2 && n == 3 && x == 3 && y == nil)

func forward(...) {
    return collect(...)
}

a, b, n, x, y = forward(7, 8, 9, 10)
assert(a == 7 && b == 8 && n == 2 && x == 9 && y == 10)

print("ok")
