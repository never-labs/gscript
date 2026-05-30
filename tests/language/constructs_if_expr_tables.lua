print("case:constructs_if_expr_tables")

local function f (i)
  if i < 10 then return "a"
  elseif i < 20 then return "b"
  elseif i < 30 then return "c"
  end
end

assert(f(3) == "a" and f(12) == "b" and f(26) == "c" and f(100) == nil)

local function g (i)
  if i < 10 then return "a"
  elseif i < 20 then return "b"
  elseif i < 30 then return "c"
  else return 8
  end
end

assert(g(3) == "a" and g(12) == "b" and g(26) == "c" and g(100) == 8)

local a, b = nil, 23
local x = {g(100) * 2 + 3 or a, a or b + 2}
assert(x[1] == 19 and x[2] == 25)

x = {f = 2 + 3 or a, a = b + 2}
assert(x.f == 5 and x.a == 25)

print("ok")
