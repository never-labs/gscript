print("case:math_floor_power")

func eq(a, b, limit) {
    limit = limit || 1E-11
    return a == b || math.abs(a - b) <= limit
}

func floordiv(a, b) {
    return math.floor(a / b)
}

assert(0e12 == 0 && 0.0 == 0 && 0.2e2 == 20 && 2.0E-1 == 0.2)

vals := {-16, -15, -3, -2, -1, 0, 1, 2, 3, 15}
divs := {-16, -15, -3, -2, -1, 1, 2, 3, 15}
for _, i := range pairs(vals) {
    for _, j := range pairs(divs) {
        assert(floordiv(i, j) == math.floor(i / j))
    }
}

assert(2 ** -3 == 1 / 2 ** 3)
assert(eq((-3) ** -3, 1 / (-3) ** 3))
for i := -3; i <= 3; i++ {
    for j := -3; j <= 3; j++ {
        if i != 0 || j > 0 {
            assert(eq(i ** j, 1 / i ** (-j)))
        }
    }
}

print("ok")
