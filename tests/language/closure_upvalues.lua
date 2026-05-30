print("case:closure_upvalues")

local A = 0
local B = {g = 10}

local function make(x)
  local funcs = {}
  for i = 1, 4 do
    local y = 0
    funcs[i] = function()
      B.g = B.g + 1
      y = y + x
      return y + A
    end
  end
  return funcs
end

local a = make(10)
assert(a[1]() == 10)
assert(a[1]() == 20)
assert(a[2]() == 10)
A = 5
assert(a[2]() == 25)
assert(B.g == 14)

local function outer(x)
  return function(y)
    return function(z)
      return x + y + z + A
    end
  end
end

assert(outer(10)(20)(30) == 65)

print("ok")
