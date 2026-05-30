print("case:sort_len_noninteger_more")

local t = setmetatable({}, {
  __len = function () return "abc" end,
})
assert(#t == "abc")

t = setmetatable({}, {
  __len = function () return -1 end,
})
assert(#t == -1)
table.sort(t, error)

print("ok")
