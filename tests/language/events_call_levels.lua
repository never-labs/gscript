print("case:events_call_levels")

local i
local tt = {
  __call = function (t, ...)
    i = i + 1
    if t.f then
      return t.f(...)
    else
      return {...}
    end
  end,
}

local a = setmetatable({}, tt)
local b = {f = a}
setmetatable(b, tt)
local c = {f = b}
setmetatable(c, tt)

i = 0
local x = c(3, 4, 5)
assert(i == 3)

print("ok")
