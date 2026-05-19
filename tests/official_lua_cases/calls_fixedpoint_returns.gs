print("case:calls_fixedpoint_returns")

Z := func(le) {
    a := nil
    a = func(f) {
        return le(func(x) { return f(f)(x) })
    }
    return a(a)
}

F := func(f) {
    return func(n) {
        if n == 0 {
            return 1
        }
        return n * f(n - 1)
    }
}

fat := Z(F)
assert(fat(0) == 1 && fat(4) == 24 && Z(F)(5) == 120)

func g(z) {
    func f(a, b, c, d) {
        return func(x, y) { return a + b + c + d + a + x + y + z }
    }
    return f(z, z + 1, z + 2, z + 3)
}

f := g(10)
assert(f(9, 16) == 10 + 11 + 12 + 13 + 10 + 9 + 16 + 10)

func unlpack(t, i) {
    i = i || 1
    if i <= #t {
        return t[i], unlpack(t, i + 1)
    }
}

a, b, c, d := unlpack({1, 2, 3})
assert(a == 1 && b == 2 && c == 3 && d == nil)

print("ok")
