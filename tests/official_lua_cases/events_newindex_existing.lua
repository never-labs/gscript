print("case:events_newindex_existing")

local fired
local a = {}
for i = 1, 10 do
  a[i] = 0
  a["a" .. i] = 0
end

setmetatable(a, {
  __newindex = function (t, k, v)
    fired = true
    rawset(t, k, v)
  end,
})

fired = false
a[1] = 0
assert(not fired)

fired = false
a["a1"] = 0
assert(not fired)

fired = false
a["a11"] = 0
assert(fired)

fired = false
a[11] = 0
assert(fired)

print("ok")
