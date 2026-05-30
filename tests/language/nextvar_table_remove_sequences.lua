print("case:nextvar_table_remove_sequences")

local a = {"c", "d"}
table.insert(a, 3, "a")
table.insert(a, "b")
assert(table.remove(a, 1) == "c")
assert(table.remove(a, 1) == "d")
assert(table.remove(a, 1) == "a")
assert(table.remove(a, 1) == "b")
assert(table.remove(a, 1) == nil)
assert(#a == 0 and a.n == nil)

a = {10, 20, 30, 40}
assert(table.remove(a, #a + 1) == nil)
assert(not pcall(table.remove, a, 0))
assert(a[#a] == 40)
assert(table.remove(a, #a) == 40)
assert(a[#a] == 30)
assert(table.remove(a, 2) == 20)
assert(a[#a] == 30 and #a == 2)

print("ok")
