print("case:events_metatable_basic_more")

assert(getmetatable{} == nil)
assert(getmetatable(4) == nil)
assert(getmetatable(nil) == nil)
local a = {name = "NAME"}
setmetatable(a, {__metatable = "xuxu", __tostring = function(x) return x.name end})
assert(getmetatable(a) == "xuxu")
assert(tostring(a) == "NAME")
assert(pcall(setmetatable, a, {}) == false)
a.name = "gororoba"
assert(tostring(a) == "gororoba")

print("ok")
