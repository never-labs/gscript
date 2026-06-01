print("case:nextvar_empty_numeric_for")

for a, b := range pairs({}) { error("not here") }
for i := 1; i <= 0; i = i + 1 { error("not here") }
for i := 0; i >= 1; i = i - 1 { error("not here") }

a := nil
for i := 1; i <= 1; i = i + 1 {
  assert(!a)
  a = 1
}
assert(a)

a = nil
for i := 1; i >= 1; i = i - 1 {
  assert(!a)
  a = 1
}
assert(a)

print("ok")
