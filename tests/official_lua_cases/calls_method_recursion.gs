print("case:calls_method_recursion")

fact := false
res := 1
factLocal := nil
factLocal = func(n) {
    if n == 0 {
        return res
    }
    return n * factLocal(n - 1)
}
assert(factLocal(5) == 120)
assert(fact == false)

a := {i: 10}
self := 20
a.x = func(selfArg, x) { return x + selfArg.i }
a.y = func(x) { return x + self }
assert(a:x(1) + 10 == a.y(1))

a.t = {i: -100}
a["t"].x = func(selfArg, aArg, b) { return selfArg.i + aArg + b }
assert(a.t:x(2, 3) == -95)

a2 := {x: 0}
a2.add = func(selfArg, x) {
    selfArg.x = selfArg.x + x
    a2.y = 20
    return selfArg
}
assert(a2:add(10):add(20):add(30).x == 60 && a2.y == 20)

print("ok")
