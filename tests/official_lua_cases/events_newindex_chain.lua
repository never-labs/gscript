print("case:events_newindex_chain")

local writes = {}
local backing = {}

local proxy = setmetatable({}, {
  __newindex = function (_, k, v)
    writes[#writes + 1] = k .. "=" .. v
    backing[k] = v
  end,
  __index = backing,
})

proxy.a = "one"
proxy.b = "two"
assert(proxy.a == "one" and proxy.b == "two")
assert(writes[1] == "a=one" and writes[2] == "b=two")

local fallback = {existing = 4}
local child = setmetatable({}, {__index = proxy, __newindex = fallback})
child.c = 9
assert(fallback.c == 9)
assert(child.a == "one")
assert(child.existing == nil)

print("ok")
