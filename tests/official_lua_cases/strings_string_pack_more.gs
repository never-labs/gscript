print("case:strings_string_pack_more")

packed := string.pack("u8 i8 i8", 0xff, 0x7f, -0x80)
u, i1, i2, pos := string.unpack("u8 i8 i8", packed)
assert(u == 0xff && i1 == 0x7f && i2 == -0x80 && pos == 4)

packed = string.pack("be:u16 u16", 0x0102, 0x0304)
assert(bytes.toHex(packed) == "01020304")
a, b, pos := string.unpack("be:u16 u16", packed)
assert(a == 0x0102 && b == 0x0304 && pos == 5)

assert(string.pack("bytes:0", "") == "")
assert(string.packsize("bytes:0") == 0)
raw, pos := string.unpack("bytes:0", "")
assert(raw == "" && pos == 1)

assert(string.pack("bytes:3", "123") == "123")
assert(string.pack("bytes:8", "123456\0\0") == "123456\0\0")

print("ok")
