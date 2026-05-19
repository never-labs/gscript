print("case:calls_multiline_adjust_more")

t := nil
func f(a, b, c) {
  d := "a"
  t = {a, b, c, d}
}

f(
  1, 2)
assert(t[1] == 1 && t[2] == 2 && t[3] == nil && t[4] == "a")
f(1, 2,
  3, 4)
assert(t[1] == 1 && t[2] == 2 && t[3] == 3 && t[4] == "a")

func h(a, b, c) {
  return a, b, c
}
a, b, c := h(
  10,
  20)
assert(a == 10 && b == 20 && c == nil)

a, b, c = h(10, 20,
  30, 40)
assert(a == 10 && b == 20 && c == 30)

print("ok")
