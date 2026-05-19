print("case:coroutine_wrap_basic")

func foo(i) {
    return coroutine.yield(i)
}

x := nil
f := coroutine.wrap(func() {
    for i := 1; i <= 10; i++ {
        assert(foo(i) == x)
    }
    return "a"
})

for i := 1; i <= 10; i++ {
    x = i
    assert(f(i) == i)
}
x = "xuxu"
assert(f("xuxu") == "a")
x = nil

func gen(n) {
    return coroutine.wrap(func() {
        for i := 2; i <= n; i++ {
            coroutine.yield(i)
        }
    })
}

iter := gen(10)
a := {}
for {
    n := iter()
    if n == nil {
        break
    }
    table.insert(a, n)
}
assert(#a == 9 && a[1] == 2 && a[#a] == 10)

print("ok")
