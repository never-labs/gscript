print("case:calls_tail_varargs_more2")

X := nil
Y := nil
A := nil
func bar(x, y, ...) { X = x; Y = y; A = {...} }
func bar1(...) { return bar(...) }
bar1()
assert(X == nil && Y == nil && #A == 0)
bar1(10)
assert(X == 10 && Y == nil && #A == 0)
bar1(10, 20)
assert(X == 10 && Y == 20 && #A == 0)

print("ok")
