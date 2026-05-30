print("case:nextvar_pairs_key_types_more")

fkey := func() { return 6 }
tkey := {}
long := string.rep("x", 1000)
a := {
  [1]: 1,
  [1.1]: 2,
  x: 3,
  [long]: 4,
  [fkey]: 5,
  [true]: 6,
  [tkey]: 7,
}
b := {}
for i := 1; i <= 7; i++ { b[i] = true }
for k, v := range pairs(a) {
  assert(b[v])
  b[v] = nil
  assert(a[k] == v)
}
assert(next(b) == nil)

n := 0
key := {}
t := {[key]: 1, [string.rep("x ", 4)]: 3, [100.3]: 4, [4]: 5}
for k, v := range pairs(t) {
  n = n + 1
  assert(t[k] == v)
  t[k] = nil
  assert(t[k] == nil)
}
assert(n == 4 && next(t) == nil)

print("ok")
