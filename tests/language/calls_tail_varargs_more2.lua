print("case:calls_tail_varargs_more2")

local X, Y, A
local function bar(x, y, ...) X = x; Y = y; A = {...} end
local function bar1(...) return bar(...) end
bar1()
assert(X == nil and Y == nil and #A == 0)
bar1(10)
assert(X == 10 and Y == nil and #A == 0)
bar1(10, 20)
assert(X == 10 and Y == 20 and #A == 0)

print("ok")
