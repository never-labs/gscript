print("case:main_multiline_chunk_values")

func f(x) {
  a := "xuxu\n"
  b := "xuxu\n"
  if x == 11 { return 1 + 12, 2 + 20 }
  return x + 1
}

assert(f(100) == 101)
a, b := f(11)
assert(a == 13 && b == 22)

print("ok")
