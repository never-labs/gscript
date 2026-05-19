print("case:events_rawget_rawset")

t := {}
rawset(t, "x", 1)
assert(rawget(t, "x") == 1)

mt := {
  __index: func(_, k) {
    return "fallback:" .. k
  },
}

setmetatable(t, mt)
assert(t.y == "fallback:y")
assert(rawget(t, "y") == nil)
assert(setmetatable(t, nil) == t)
assert(getmetatable(t) == nil)

print("ok")
