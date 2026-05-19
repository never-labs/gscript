print("case:events_newindex_existing")

fired := nil
a := {}
for i := 1; i <= 10; i++ {
  a[i] = 0
  a["a" .. i] = 0
}

setmetatable(a, {
  __newindex: func(t, k, v) {
    fired = true
    rawset(t, k, v)
  },
})

fired = false
a[1] = 0
assert(!fired)

fired = false
a["a1"] = 0
assert(!fired)

fired = false
a["a11"] = 0
assert(fired)

fired = false
a[11] = 0
assert(fired)

print("ok")
