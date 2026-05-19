print("case:nextvar_table_insert_remove_proxy_metamethods")

local backingInsert = {[1] = "a", [2] = "c"}
local insertReads, insertWrites = 0, 0
local proxyInsert = setmetatable({}, {
  __len = function() return 2 end,
  __index = function(_, k)
    insertReads = insertReads + 1
    return backingInsert[k]
  end,
  __newindex = function(_, k, v)
    insertWrites = insertWrites + 1
    backingInsert[k] = v
  end,
})
table.insert(proxyInsert, 2, "b")
assert(backingInsert[1] == "a" and backingInsert[2] == "b" and backingInsert[3] == "c")
table.insert(proxyInsert, "d")
assert(backingInsert[3] == "d")
assert(insertReads >= 1 and insertWrites >= 2)
assert(next(proxyInsert) == nil)

local backingRemove = {[1] = "a", [2] = "b", [3] = "c"}
local removeReads, removeWrites = 0, 0
local proxyRemove = setmetatable({}, {
  __len = function() return 3 end,
  __index = function(_, k)
    removeReads = removeReads + 1
    return backingRemove[k]
  end,
  __newindex = function(_, k, v)
    removeWrites = removeWrites + 1
    backingRemove[k] = v
  end,
})
assert(table.remove(proxyRemove, 2) == "b")
assert(backingRemove[1] == "a" and backingRemove[2] == "c" and backingRemove[3] == nil)
assert(table.remove(proxyRemove, 4) == nil)
assert(removeReads >= 1 and removeWrites >= 1)
assert(next(proxyRemove) == nil)

print("ok")
