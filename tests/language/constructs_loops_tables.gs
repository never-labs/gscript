print("case:constructs_loops_tables")

f := func(i) {
    if i < 10 {
        return "a"
    } elseif i < 20 {
        return "b"
    } elseif i < 30 {
        return "c"
    }
    return 8
}

assert(f(3) == "a" && f(12) == "b" && f(26) == "c" && f(100) == 8)

n := 100
i := 3
t := {}
a := nil
for !a {
    a = 0
    for i := 1; i <= n; i++ {
        for j := i; j >= 1; j-- {
            a = a + 1
            t[j] = 1
        }
    }
}
assert(a == n * (n + 1) / 2 && i == 3)
assert(t[1] && t[n] && t[n + 1] == nil)

a2, b := nil, 23
x := {f(100) * 2 + 3 || a2, a2 || b + 2}
assert(x[1] == 19 && x[2] == 25)
x = {f: 2 + 3 || a2, a: b + 2}
assert(x.f == 5 && x.a == 25)

print("ok")
