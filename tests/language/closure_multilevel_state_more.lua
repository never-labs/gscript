print("case:closure_multilevel_state_more")

local w

local function f(x)
  return function(y)
    return function(z)
      return w + x + y + z
    end
  end
end

local y = f(10)
w = 1.345
assert(y(20)(30) == 60 + w)

local function make(x)
  local a = "xuxu"
  return function(op, y)
    if op == "set" then
      a = x + y
    else
      return a
    end
  end
end

local b1 = make(1)
local b2 = make(4)
assert(b1("get") == "xuxu" and b2("get") == "xuxu")
b1("set", 10)
b2("set", 10)
assert(b1("get") == 11 and b2("get") == 14)

print("ok")
