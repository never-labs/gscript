print("case:vararg_long_parameter_list_more")

seen := false
func f(p1, p2, p3, p4, p5, p6, p7, p8, p9, p10,
p11, p12, p13, p14, p15, p16, p17, p18, p19, p20,
p21, p22, p23, p24, p25, p26, p27, p28, p29, p30,
p31, p32, p33, p34, p35, p36, p37, p38, p39, p40,
p41, p42, p43, p44, p45, p46, p48, p49, p50, ...) {
  assert(p1 == nil && p20 == nil && p50 == nil)
  assert(select("#", ...) == 0)
  seen = true
}
f()
assert(seen)

func g(p1, p2, p3, p4, p5, p6, p7, p8, p9, p10,
p11, p12, p13, p14, p15, p16, p17, p18, p19, p20,
p21, p22, p23, p24, p25, p26, p27, p28, p29, p30,
p31, p32, p33, p34, p35, p36, p37, p38, p39, p40,
p41, p42, p43, p44, p45, p46, p48, p49, p50, ...) {
  return p1, p25, p50, select("#", ...), ...
}
a, b, c, n, x, y := g(1, 2, 3, 4, 5, 6, 7, 8, 9, 10,
11, 12, 13, 14, 15, 16, 17, 18, 19, 20,
21, 22, 23, 24, 25, 26, 27, 28, 29, 30,
31, 32, 33, 34, 35, 36, 37, 38, 39, 40,
41, 42, 43, 44, 45, 46, 48, 49, 50, 51, 52)
assert(a == 1 && b == 25 && c == 50 && n == 2 && x == 51 && y == 52)

print("ok")
