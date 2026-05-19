print("case:sort_equal_false_more")

a := {}
for i := 1; i <= 32; i++ { a[i] = false }
table.sort(a, func(x, y) { return nil })
for i, v := range pairs(a) { assert(v == false) }
assert(#a == 32)

table.sort({})

print("ok")
