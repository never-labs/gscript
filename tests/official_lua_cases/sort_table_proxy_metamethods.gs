print("case:sort_table_proxy_metamethods")

src := {[1]: "a", [2]: "b", [3]: "c"}
dst := {}
reads, writes := 0, 0
proxySrc := setmetatable({}, {
  __index: func(_, k) {
    reads = reads + 1
    return src[k]
  },
})
proxyDst := setmetatable({}, {
  __newindex: func(_, k, v) {
    writes = writes + 1
    dst[k] = v
  },
})
assert(table.move(proxySrc, 1, 3, 2, proxyDst) == proxyDst)
assert(dst[1] == nil && dst[2] == "a" && dst[3] == "b" && dst[4] == "c")
assert(reads == 3 && writes == 3)

unpacked := setmetatable({}, {
  __len: func() { return 3 },
  __index: func(_, k) { return src[k] },
})
a, b, c := table.unpack(unpacked)
assert(a == "a" && b == "b" && c == "c")

backingSort := {[1]: 3, [2]: 1, [3]: 2}
proxySort := setmetatable({}, {
  __len: func() { return 3 },
  __index: func(_, k) { return backingSort[k] },
  __newindex: func(_, k, v) { rawset(backingSort, k, v) },
})
table.sort(proxySort)
assert(backingSort[1] == 1 && backingSort[2] == 2 && backingSort[3] == 3)
assert(next(proxySort) == nil)

print("ok")
