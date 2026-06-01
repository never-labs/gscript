print("case:events_index_chain")

a := nil
a = setmetatable({}, {
  __index: setmetatable({}, {
    __index: setmetatable({}, {
      __index: func(_, n) {
        return a[n - 3] + 4
      },
    }),
  }),
})

a[0] = 20
for i := 0; i <= 10; i++ {
  assert(a[i * 3] == 20 + i * 4)
}

print("ok")
