print("case:nextvar_table_insert_remove_proxy_metamethods")

backingInsert := {[1]: "a", [2]: "c"}
insertReads, insertWrites := 0, 0
proxyInsert := setmetatable({}, {
  __len: func() { return 2 },
  __index: func(_, k) {
    insertReads = insertReads + 1
    return backingInsert[k]
  },
  __newindex: func(_, k, v) {
    insertWrites = insertWrites + 1
    backingInsert[k] = v
  },
})
table.insert(proxyInsert, 2, "b")
assert(backingInsert[1] == "a" && backingInsert[2] == "b" && backingInsert[3] == "c")
table.insert(proxyInsert, "d")
assert(backingInsert[3] == "d")
assert(insertReads >= 1 && insertWrites >= 2)
assert(next(proxyInsert) == nil)

backingRemove := {[1]: "a", [2]: "b", [3]: "c"}
removeReads, removeWrites := 0, 0
proxyRemove := setmetatable({}, {
  __len: func() { return 3 },
  __index: func(_, k) {
    removeReads = removeReads + 1
    return backingRemove[k]
  },
  __newindex: func(_, k, v) {
    removeWrites = removeWrites + 1
    backingRemove[k] = v
  },
})
assert(table.remove(proxyRemove, 2) == "b")
assert(backingRemove[1] == "a" && backingRemove[2] == "c" && backingRemove[3] == nil)
assert(table.remove(proxyRemove, 4) == nil)
assert(removeReads >= 1 && removeWrites >= 1)
assert(next(proxyRemove) == nil)

print("ok")
