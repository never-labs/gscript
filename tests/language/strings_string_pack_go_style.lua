print("case:strings_string_pack_go_style")

packed = string.pack(">I2 c2", 258, "go")
a, raw, next = string.unpack(">I2 c2", packed)
assert(a == 258 and raw == "go" and next == 5)
assert(string.packsize(">I2 c2") == 4)

print("ok")
