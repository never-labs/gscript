print("case:code_string_constant_closure_more")

local k0 = "00000000000000000000000000000000000000"
local function f1()
  local k = k0
  return function()
    return function() return k end
  end
end

local f2 = f1()
local f3 = f2()
assert(f3() == k0)
assert(string.len(f3()) == string.len(k0))

print("ok")
