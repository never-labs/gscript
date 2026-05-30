print("case:sort_table_proxy_metamethods")

local src = {[1] = "a", [2] = "b", [3] = "c"}
local dst = {}
local reads, writes = 0, 0
local proxySrc = setmetatable({}, {
  __index = function(_, k)
    reads = reads + 1
    return src[k]
  end,
})
local proxyDst = setmetatable({}, {
  __newindex = function(_, k, v)
    writes = writes + 1
    dst[k] = v
  end,
})
assert(table.move(proxySrc, 1, 3, 2, proxyDst) == proxyDst)
assert(dst[1] == nil and dst[2] == "a" and dst[3] == "b" and dst[4] == "c")
assert(reads == 3 and writes == 3)

local unpacked = setmetatable({}, {
  __len = function() return 3 end,
  __index = function(_, k) return src[k] end,
})
local a, b, c = table.unpack(unpacked)
assert(a == "a" and b == "b" and c == "c")

local backingSort = {[1] = 3, [2] = 1, [3] = 2}
local proxySort = setmetatable({}, {
  __len = function() return 3 end,
  __index = function(_, k) return backingSort[k] end,
  __newindex = function(_, k, v) rawset(backingSort, k, v) end,
})
table.sort(proxySort)
assert(backingSort[1] == 1 and backingSort[2] == 2 and backingSort[3] == 3)
assert(next(proxySort) == nil)

print("ok")
