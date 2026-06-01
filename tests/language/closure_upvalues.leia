print("case:closure_upvalues")

A := 0
B := {g: 10}

func make(x) {
    funcs := {}
    for i := 1; i <= 4; i++ {
        y := 0
        funcs[i] = func() {
            B.g = B.g + 1
            y = y + x
            return y + A
        }
    }
    return funcs
}

a := make(10)
assert(a[1]() == 10)
assert(a[1]() == 20)
assert(a[2]() == 10)
A = 5
assert(a[2]() == 25)
assert(B.g == 14)

func outer(x) {
    return func(y) {
        return func(z) {
            return x + y + z + A
        }
    }
}

assert(outer(10)(20)(30) == 65)

print("ok")
