print("case:binary_namespace_more")

packed := binary.pack("be:u16 i8 bytes:2 string", 258, -7, "go", "hi")
hex := bytes.toHex(packed)
assert(hex == "0102f9676f000000026869")

a, b, raw, s, next := binary.unpack("be:u16 i8 bytes:2 string", packed)
assert(a == 258 && b == -7 && raw == "go" && s == "hi" && next == 12)

le := binary.pack("le u16 u32", 513, 16909060)
x, y, off := binary.unpack("le u16 u32", "xx" .. le, 3)
assert(x == 513 && y == 16909060 && off == 9)

fixed := binary.size("be:u16 i8 bytes:2")
variable, variableErr := binary.size("string")
bad, badErr := binary.unpack("u32", "a")
assert(fixed == 5)
assert(variable == nil && type(variableErr) == "string")
assert(bad == nil && type(badErr) == "string")

print("ok")
