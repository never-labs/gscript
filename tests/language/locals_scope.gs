print("case:locals_scope")

func f1(x) {
    x = nil
    return x
}
assert(f1(10) == nil)

func f2() {
    x := nil
    return x
}
assert(f2(10) == nil)

func f3(x) {
    x = nil
    y := nil
    return x, y
}
assert(f3(10) == nil && select(2, f3(20)) == nil)

iOuter := 10
iInner := 100
assert(iInner == 100)
iInner = 1000
assert(iInner == 1000)
assert(iOuter == 10)
if iOuter != 10 {
    iBranch := 20
    _ = iBranch
} else {
    iBranch := 30
    assert(iBranch == 30)
}

x := 1
func f(a) {
    x := 3
    b := a
    c, d := a, b
    _ = c
    if d == b {
        x2 := "q"
        x2 = b
        assert(x2 == 2)
    } else {
        assert(nil)
    }
    assert(x == 3)
}

b := 10
_ = b
a := nil
for {
    b2 := nil
    a, b2 = 1, 2
    assert(a + 1 == b2)
    if a + b2 == 3 {
        break
    }
}

assert(x == 1)
f(2)
assert(type(f) == "function")

print("ok")
