print("case:events_index_function_parent_more")

local a = {10, 20, 30; x = "10", y = "20"}
local t = {}

local function f(tbl, i, e)
  assert(not e)
  local p = rawget(tbl, "parent")
  return (p and p[i] + 3), "dummy return"
end

t.__index = f
a.parent = {z = 25, x = 12, [4] = 24}
setmetatable(a, t)
assert(a[1] == 10 and a.z == 28 and a[4] == 27 and a.x == "10")

local called = false
local b = {}
setmetatable(b, {
  __index = function(tbl, key, extra)
    assert(tbl == b and extra == nil)
    called = key == "missing"
    return "fallback"
  end,
})
assert(b.missing == "fallback" and called)

print("ok")
