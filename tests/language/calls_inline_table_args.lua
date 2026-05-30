print("case:calls_inline_table_args")

local a = table.move({10, 20, 30}, 1, 3, 2)
assert(a[1] == 10 and a[2] == 10 and a[3] == 20 and a[4] == 30)

a = table.move({10, 20, 30}, 2, 3, 1)
assert(a[1] == 20 and a[2] == 30 and a[3] == 30)

assert(table.unpack({"x", "y"}, 1, 2) == "x")

print("ok")
