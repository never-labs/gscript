print("case:events_concat_chain_more")

local t = {}
t.__concat = function(a, b)
  if type(a) == 'table' then a = a.val end
  if type(b) == 'table' then b = b.val end
  return setmetatable({val = a .. b}, t)
end

local c = setmetatable({val = "c"}, t)
local d = setmetatable({val = "d"}, t)
assert((c .. d .. c .. d).val == "cdcd")
local x = c .. d
assert(getmetatable(x) == t and x.val == "cd")
x = 0 .. "a" .. "b" .. c .. d .. "e" .. "f" .. "g"
assert(x.val == "0abcdefg")

print("ok")
