print("case:api_raw_ops_more")

local log = {}
local t = setmetatable({x = 1}, {
  __index = function(_, k)
    log[#log + 1] = "index:" .. k
    return 20
  end,
  __newindex = function(_, k, v)
    log[#log + 1] = "new:" .. k .. ":" .. v
  end,
})

assert(t.x == 1)
assert(t.y == 20)
assert(log[1] == "index:y")

t.y = 30
assert(rawget(t, "y") == nil)
assert(log[2] == "new:y:30")

rawset(t, "y", 40)
assert(t.y == 40)
assert(rawget(t, "y") == 40)

local mt = getmetatable(t)
assert(type(mt) == "table")
assert(setmetatable(t, nil) == t)
assert(getmetatable(t) == nil)

print("ok")
