print("case:calls_tail_missing_matrix_more")

local function f(a,b,c,d) return a,b,c,d end
local function g0() return f() end
local function g1() return f(1) end
local function g2() return f(1,2) end
local function g3() return f(1,2,3) end

local a,b,c,d = g0()
assert(a == nil and b == nil and c == nil and d == nil)
a,b,c,d = g1()
assert(a == 1 and b == nil and c == nil and d == nil)
a,b,c,d = g2()
assert(a == 1 and b == 2 and c == nil and d == nil)
a,b,c,d = g3()
assert(a == 1 and b == 2 and c == 3 and d == nil)

local function h() return f(1,2) end
a,b = h()
assert(a == 1 and b == 2)

print("ok")
