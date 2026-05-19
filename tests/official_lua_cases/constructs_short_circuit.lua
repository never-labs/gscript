print("case:constructs_short_circuit")

local function f (a, b, c, d, e)
  local x = a >= b or c or (d and e) or nil
  return x
end

local function g (a, b, c, d, e)
  if not (a >= b or c or d and e or nil) then
    return 0
  else
    return 1
  end
end

local function h (a, b, c, d, e)
  while (a >= b or c or (d and e) or nil) do
    return 1
  end
  return 0
end

assert(f(2, 1) == true and g(2, 1) == 1 and h(2, 1) == 1)
assert(f(1, 2, "a") == "a" and g(1, 2, "a") == 1 and h(1, 2, "a") == 1)
assert(f(1, 2, nil, 1, "x") == "x" and g(1, 2, nil, 1, "x") == 1 and h(1, 2, nil, 1, "x") == 1)
assert(f(1, 2, nil, nil, "x") == nil and g(1, 2, nil, nil, "x") == 0 and h(1, 2, nil, nil, "x") == 0)
assert(f(1, 2, nil, 1, nil) == nil and g(1, 2, nil, 1, nil) == 0 and h(1, 2, nil, 1, nil) == 0)

assert(1 and 2 < 3 == true and 2 < 3 and "a" < "b" == true)
local x = 2 < 3 and not 3
assert(x == false)
x = 2 < 1 or (2 > 1 and "a")
assert(x == "a")

print("ok")
