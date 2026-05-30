print("case:vararg_select")

local a, b = select(3, 10, 20, 30, 40)
assert(a == 30 and b == 40)
a = select(1)
assert(a == nil)
a, b = select(-1, 3, 5, 7)
assert(a == 7 and b == nil)
a, b, c = select(-2, 3, 5, 7)
assert(a == 5 and b == 7 and c == nil)
pcall(select, 10000)
pcall(select, -10000)

print("ok")
