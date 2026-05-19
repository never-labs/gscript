print("case:nextvar_hash_length_growth")

a := {}

for i := 1; i <= 100; i = i + 1 {
  a[i .. "+"] = true
}
for i := 1; i <= 100; i = i + 1 {
  a[i .. "+"] = nil
}

for i := 1; i <= 100; i = i + 1 {
  a[i] = true
  assert(#a == i)
}

assert(a["1+"] == nil)
assert(a["100+"] == nil)
assert(a[1] == true)
assert(a[100] == true)

print("ok")
