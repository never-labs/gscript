print("case:sort_len_noninteger_more")

t := setmetatable({}, {
  __len: func() { return "abc" },
})
assert(#t == "abc")

t = setmetatable({}, {
  __len: func() { return -1 },
})
assert(#t == -1)
table.sort(t, error)

print("ok")
