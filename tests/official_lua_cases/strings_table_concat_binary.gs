print("case:strings_table_concat_binary")

z := string.char(0)
z1 := string.char(0, 1)
z12 := string.char(0, 1, 2)
parts := {z, z1, z12}
assert(table.concat(parts, "." .. z .. ".") ==
       z .. "." .. z .. "." .. z1 .. "." .. z .. "." .. z12)

a := {}
for i := 1; i <= 300; i++ {
  a[i] = "xuxu"
}
assert(table.concat(a, "123") .. "123" == string.rep("xuxu123", 300))
assert(table.concat(a, "b", 20, 20) == "xuxu")
assert(table.concat(a, "", 20, 21) == "xuxuxuxu")
assert(table.concat(a, "x", 22, 21) == "")
assert(table.concat(a, "3", 299) == "xuxu3xuxu")

print("ok")
