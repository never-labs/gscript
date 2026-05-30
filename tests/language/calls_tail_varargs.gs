print("case:calls_tail_varargs")

X := nil
Y := nil
A := nil
func sink(x, y, ...) {
  X = x
  Y = y
  A = {...}
}

func forward(...) {
  return sink(...)
}

a, b, c := forward()
assert(X == nil && Y == nil && #A == 0)

print("ok")
