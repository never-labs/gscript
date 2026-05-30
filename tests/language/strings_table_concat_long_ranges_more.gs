print("case:strings_table_concat_long_ranges_more")

a := {}
for i := 1; i <= 300; i++ {
  a[i] = "xuxu"
}

assert(table.concat(a, "123") .. "123" == string.rep("xuxu123", 300))
assert(table.concat(a, "b", 20, 20) == "xuxu")
assert(table.concat(a, "", 20, 21) == "xuxuxuxu")
assert(table.concat(a, "x", 22, 21) == "")
assert(table.concat(a, "3", 299) == "xuxu3xuxu")

b := {"a", "b", "c"}
assert(table.concat(b, ",", 1, 0) == "")
assert(table.concat(b, ",", 1, 1) == "a")
assert(table.concat(b, ",", 1, 2) == "a,b")
assert(table.concat(b, ",", 2) == "b,c")
assert(table.concat(b, ",", 3) == "c")
assert(table.concat(b, ",", 4) == "")

print("ok")
