print("case:events_concat_metamethod")

local mt = {}
mt.__concat = function (a, b)
  if type(a) == "table" then
    a = a.val
  end
  if type(b) == "table" then
    b = b.val
  end
  return a .. b
end

local c = {val = "c"}
local d = {val = "d"}
setmetatable(c, mt)
setmetatable(d, mt)

assert(c .. d == "cd")
assert(0 .. "a" .. "b" .. c .. d .. "e" .. "f" .. (5 + 3) .. "g" == "0abcdef8g")

print("ok")
