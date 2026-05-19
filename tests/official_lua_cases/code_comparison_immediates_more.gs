print("case:code_comparison_immediates_more")

func eq1(a) { if a == 1 { return 2 } }
func eqs(a) { if a == "hi" { return 2 } }
func le1(a) { if -10 <= a { return 2 } }
func lt1(a) { if 10 < a { return 2 } }
func ge1(a) { if a >= 23.0 { return 2 } }

assert(eq1(1) == 2 && eq1(0) == nil)
assert(eqs("hi") == 2 && eqs("bye") == nil)
assert(le1(-10) == 2 && le1(-11) == nil)
assert(lt1(11) == 2 && lt1(10) == nil)
assert(ge1(25) == 2 && ge1(22) == nil)

print("ok")
