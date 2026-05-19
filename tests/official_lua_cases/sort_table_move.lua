print("case:sort_table_move")

local a = table.move({10, 20, 30}, 1, 3, 2)
assert(a[1] == 10 and a[2] == 10 and a[3] == 20 and a[4] == 30)

a = table.move({10, 20, 30}, 1, 3, 3)
assert(a[1] == 10 and a[2] == 20 and a[3] == 10 and a[4] == 20 and a[5] == 30)

a = {10, 20, 30, 40}
table.move(a, 1, 4, 2, a)
assert(a[1] == 10 and a[2] == 10 and a[3] == 20 and a[4] == 30 and a[5] == 40)

a = table.move({10, 20, 30}, 2, 3, 1)
assert(a[1] == 20 and a[2] == 30 and a[3] == 30)

a = {}
assert(table.move({10, 20, 30}, 1, 3, 1, a) == a)
assert(a[1] == 10 and a[2] == 20 and a[3] == 30)

a = {}
assert(table.move({10, 20, 30}, 1, 0, 3, a) == a)
assert(next(a) == nil)

a = table.move({10, 20, 30}, 1, 10, 1)
assert(a[1] == 10 and a[2] == 20 and a[3] == 30)

print("ok")
