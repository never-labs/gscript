print("case:nextvar_pairs_many_string_keys")

a := {}
for i := 0; i <= 10000; i++ {
  if math.fmod(i, 10) != 0 {
    a["x" .. i] = i
  }
}

n := {n: 0}
for k, v := range pairs(a) {
  n.n = n.n + 1
  assert(k && v && a[k] == v)
}

assert(n.n == 9000)

print("ok")
