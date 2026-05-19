print("case:calls_call_chain")

n := 5
u := table.pack
for i := 1; i <= n; i++ {
  t := {}
  t[1] = i
  mt := {}
  mt.__call = u
  u = setmetatable(t, mt)
}

res := u("a", "b", "c")
assert(res.n == n + 3)
for i := 1; i <= n; i++ {
  assert(res[i][1] == i)
}
assert(res[n + 1] == "a" && res[n + 2] == "b" && res[n + 3] == "c")

print("ok")
