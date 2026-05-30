print("case:calls_tail_missing_matrix_more")

func f(a, b, c, d) { return a, b, c, d }
func g0() { return f() }
func g1() { return f(1) }
func g2() { return f(1, 2) }
func g3() { return f(1, 2, 3) }

a, b, c, d := g0()
assert(a == nil && b == nil && c == nil && d == nil)
a, b, c, d = g1()
assert(a == 1 && b == nil && c == nil && d == nil)
a, b, c, d = g2()
assert(a == 1 && b == 2 && c == nil && d == nil)
a, b, c, d = g3()
assert(a == 1 && b == 2 && c == 3 && d == nil)

func h() { return f(1, 2) }
a, b = h()
assert(a == 1 && b == 2)

print("ok")
