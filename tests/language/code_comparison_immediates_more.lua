print("case:code_comparison_immediates_more")

local function eq1(a) if a == 1 then return 2 end end
local function eqs(a) if a == "hi" then return 2 end end
local function le1(a) if -10 <= a then return 2 end end
local function lt1(a) if 10 < a then return 2 end end
local function ge1(a) if a >= 23.0 then return 2 end end

assert(eq1(1) == 2 and eq1(0) == nil)
assert(eqs("hi") == 2 and eqs("bye") == nil)
assert(le1(-10) == 2 and le1(-11) == nil)
assert(lt1(11) == 2 and lt1(10) == nil)
assert(ge1(25) == 2 and ge1(22) == nil)

print("ok")
