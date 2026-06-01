// Conformance hot benchmark: events/metamethod dispatch.
// Covers __index/__newindex, method dispatch, arithmetic/compare, and concat.

MOD := 1000000007
N := 600000

methods := {}

func proxy_index(obj, key) {
    if key == "step" {
        return methods.step
    }
    if key == "mix" {
        return methods.mix
    }
    slots := rawget(obj, "slots")
    value := slots[key]
    if value != nil {
        return value
    }
    return rawget(obj, "base") + #key
}

func proxy_newindex(obj, key, value) {
    slots := rawget(obj, "slots")
    slots[key] = value
}

proxy_mt := {
    __index: proxy_index,
    __newindex: proxy_newindex,
}

methods.step = func(self, delta) {
    self.accum = self.accum + delta + self.bias
    return self.accum
}

methods.mix = func(self, i) {
    return self:step(i % 17) + self.shadow
}

func new_proxy(base) {
    obj := {base: base, slots: {accum: 0, bias: 3, shadow: 11}}
    return setmetatable(obj, proxy_mt)
}

num_mt := {}

func new_num(v) {
    return setmetatable({v: v}, num_mt)
}

num_mt.__add = func(a, b) {
    return a.v + b.v
}

num_mt.__sub = func(a, b) {
    return a.v - b.v
}

num_mt.__mul = func(a, b) {
    return a.v * b.v
}

num_mt.__unm = func(a) {
    return -a.v
}

num_mt.__lt = func(a, b) {
    return a.v < b.v
}

num_mt.__le = func(a, b) {
    return a.v <= b.v
}

str_mt := {}

str_mt.__concat = func(a, b) {
    if type(a) == "table" {
        a = a.s
    }
    if type(b) == "table" {
        b = b.s
    }
    return a .. b
}

func new_str(s) {
    return setmetatable({s: s}, str_mt)
}

func run_events(n) {
    p := new_proxy(23)
    a := new_num(7)
    b := new_num(5)
    c := new_num(12)
    sa := new_str("a")
    sb := new_str("b")
    checksum := 0

    for i := 1; i <= n; i++ {
        v := p:mix(i)
        checksum = (checksum + v + p.missing + p.accum) % MOD

        arith := (a + b) * (c - b) + (-b)
        if arith != nil {
            checksum = (checksum + 13) % MOD
        }
        if a < c {
            checksum = (checksum + i % 97) % MOD
        }
        if b <= a {
            checksum = (checksum + 31) % MOD
        }
        if i % 16 == 0 {
            s := sa .. sb
            checksum = (checksum + #s) % MOD
        }
    }

    return checksum
}

warm := run_events(2000)

t0 := time.now()
checksum := (run_events(N) + warm) % MOD
elapsed := time.since(t0)

print(string.format("events_metamethod_hot(%d): checksum: %d", N, checksum))
print(string.format("Time: %.3fs", elapsed))
