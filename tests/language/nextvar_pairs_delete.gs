print("case:nextvar_pairs_delete")

k1 := {1}
k2 := {2}
t := {}
t[k1] = 1
t[k2] = 2
t[string.rep("x ", 4)] = 3
t[100.3] = 4
t[4] = 5

n := 0
for k, v := range pairs(t) {
  n = n + 1
  assert(t[k] == v)
  t[k] = nil
  assert(t[k] == nil)
}

assert(n == 5)
assert(next(t) == nil)

print("ok")
