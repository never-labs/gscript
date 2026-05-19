print("case:events_rawget_rawset")

local t = {}
rawset(t, "x", 1)
assert(rawget(t, "x") == 1)

local mt = {
  __index = function (_, k)
    return "fallback:" .. k
  end,
}

setmetatable(t, mt)
assert(t.y == "fallback:y")
assert(rawget(t, "y") == nil)
assert(setmetatable(t, nil) == t)
assert(getmetatable(t) == nil)

print("ok")
