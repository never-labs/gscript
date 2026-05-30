print("case:events_metatable_protection_more")

assert(getmetatable{} == nil)
assert(getmetatable(4) == nil)
assert(getmetatable(nil) == nil)

local a = {name = "NAME"}
setmetatable(a, {
  __metatable = "xuxu",
  __tostring = function(x) return x.name end,
})

assert(getmetatable(a) == "xuxu")
assert(tostring(a) == "NAME")
assert(pcall(setmetatable, a, {}) == false)
a.name = "gororoba"
assert(tostring(a) == "gororoba")

local mt = {}
local b = {10, 20, 30; x = "10", y = "20"}
assert(setmetatable(b, mt) == b)
assert(getmetatable(b) == mt)
assert(setmetatable(b, nil) == b)
assert(getmetatable(b) == nil)
assert(setmetatable(b, mt) == b)

print("ok")
