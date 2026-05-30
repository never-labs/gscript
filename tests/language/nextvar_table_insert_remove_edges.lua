print("case:nextvar_table_insert_remove_edges")

local function test (a)
  assert(not pcall(table.insert, a, 2, 20))
  table.insert(a, 10)
  table.insert(a, 2, 20)
  table.insert(a, 1, -1)
  table.insert(a, 40)
  table.insert(a, #a + 1, 50)
  table.insert(a, 2, -2)
  assert(a[2] ~= nil)
  assert(a["2"] == nil)
  assert(not pcall(table.insert, a, 0, 20))
  assert(not pcall(table.insert, a, #a + 2, 20))
  assert(table.remove(a, 1) == -1)
  assert(table.remove(a, 1) == -2)
  assert(table.remove(a, 1) == 10)
  assert(table.remove(a, 1) == 20)
  assert(table.remove(a, 1) == 40)
  assert(table.remove(a, 1) == 50)
  assert(table.remove(a, 1) == nil)
  assert(table.remove(a) == nil)
  assert(table.remove(a, #a) == nil)
end

local a = {n = 0, [-7] = "ban"}
test(a)
assert(a.n == 0 and a[-7] == "ban")

a = {[-7] = "ban"}
test(a)
assert(a.n == nil and #a == 0 and a[-7] == "ban")

a = {[-1] = "ban"}
test(a)
assert(#a == 0 and table.remove(a) == nil and a[-1] == "ban")

a = {[0] = "ban"}
assert(#a == 0 and table.remove(a) == "ban" and a[0] == nil)

table.insert(a, 1, 10)
table.insert(a, 1, 20)
table.insert(a, 1, -1)
assert(table.remove(a) == 10)
assert(table.remove(a) == 20)
assert(table.remove(a) == -1)
assert(table.remove(a) == nil)

print("ok")
