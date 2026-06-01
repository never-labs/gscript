print("case:binary_numeric_fields_more")

func close(a, b, eps) {
	return math.abs(a - b) <= eps
}

packed := binary.pack("< i16 i32 i64 u8 u32 u64 f32 f64",
	-0x1234, -0x1234567, -0x123456789ab, 0xfe, 0x89abcdef, 0x123456789ab, 1.5, -2.25)
assert(bytes.toHex(packed) == "cced99badcfe557698badcfefffffeefcdab89ab896745230100000000c03f00000000000002c0")
a, b, c, d, e, f, g, h, pos := binary.unpack("< i16 i32 i64 u8 u32 u64 f32 f64", packed)
assert(a == -0x1234 && b == -0x1234567 && c == -0x123456789ab)
assert(d == 0xfe && e == 0x89abcdef && f == 0x123456789ab)
assert(close(g, 1.5, 0.000001) && close(h, -2.25, 0.000001) && pos == 40)
assert(binary.size("i16 i32 i64 u8 u32 u64 f32 f64") == 39)

big := binary.pack(">int16 uint32", -2, 0x01020304)
assert(bytes.toHex(big) == "fffe01020304")
x, y, next := binary.unpack(">int16 uint32", "zz" .. big, 3)
assert(x == -2 && y == 0x01020304 && next == 9)

assert(bytes.toHex(binary.pack(">u32", 0x01020304)) == "01020304")
assert(bytes.toHex(binary.pack("<u32", 0x01020304)) == "04030201")

spacked := string.pack("> i16 i32 i64 u8 u32 u64 f32 f64",
	-0x1234, -0x1234567, -0x123456789ab, 0xfe, 0x89abcdef, 0x123456789ab, 1.5, -2.25)
assert(bytes.toHex(spacked) == "edccfedcba99fffffedcba987655fe89abcdef00000123456789ab3fc00000c002000000000000")
sa, sb, sc, sd, se, sf, sg, sh, spos := string.unpack("> i16 i32 i64 u8 u32 u64 f32 f64", spacked)
assert(sa == -0x1234 && sb == -0x1234567 && sc == -0x123456789ab)
assert(sd == 0xfe && se == 0x89abcdef && sf == 0x123456789ab)
assert(close(sg, 1.5, 0.000001) && close(sh, -2.25, 0.000001) && spos == 40)
assert(string.packsize("int16 uint32 f32 f64") == 18)

short, shortErr := binary.unpack("u32", "\1\2\3")
assert(short == nil && string.find(shortErr, "data too short", 1, true) != nil)
soff, soffErr := string.unpack("u8", "abc", 0)
assert(soff == nil && string.find(soffErr, "offset out of range", 1, true) != nil)

ok, err := pcall(binary.pack, "i16", 32768)
assert(!ok && string.find(err, "out of range", 1, true) != nil)
ok, err = pcall(string.pack, "u8", -1)
assert(!ok && string.find(err, "negative value for unsigned field", 1, true) != nil)
ok, err = pcall(binary.pack, "u8", 256)
assert(!ok && string.find(err, "out of range", 1, true) != nil)

print("ok")
