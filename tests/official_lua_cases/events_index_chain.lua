print("case:events_index_chain")

local a
a = setmetatable({}, {
  __index = setmetatable({}, {
    __index = setmetatable({}, {
      __index = function (_, n)
        return a[n - 3] + 4
      end,
    }),
  }),
})

a[0] = 20
for i = 0, 10 do
  assert(a[i * 3] == 20 + i * 4)
end

print("ok")
