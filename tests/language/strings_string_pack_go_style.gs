print("case:strings_string_pack_go_style")

packed := string.pack("be:u16 bytes:2", 258, "go")
assert(bytes.toHex(packed) == "0102676f")

a, raw, next := string.unpack("be:u16 bytes:2", packed)
assert(a == 258 && raw == "go" && next == 5)

assert(string.packsize("be:u16 bytes:2") == 4)
n, err := string.packsize("string")
assert(n == nil && err != nil)

print("ok")
