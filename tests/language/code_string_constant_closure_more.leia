print("case:code_string_constant_closure_more")

k0 := "00000000000000000000000000000000000000"
func f1() {
  k := k0
  return func() {
    return func() { return k }
  }
}

f2 := f1()
f3 := f2()
assert(f3() == k0)
assert(string.len(f3()) == string.len(k0))

print("ok")
