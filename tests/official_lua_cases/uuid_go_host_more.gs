print("case:uuid_go_host_more")

nilUUID := uuid["nil"]()
assert(nilUUID == "00000000-0000-0000-0000-000000000000")
assert(uuid.isValid(nilUUID))
assert(uuid.isValid("123E4567-E89B-42D3-A456-426614174000"))
assert(!uuid.isValid("not-a-uuid"))

parsed, err := uuid.parse("123e4567-e89b-42d3-a456-426614174000")
assert(err == nil)
assert(parsed.version == 4)
assert(parsed.variant == "RFC4122")
assert(parsed.bytes == "123e4567e89b42d3a456426614174000")

bad, err := uuid.parse("bad")
assert(bad == nil)
assert(type(err) == "string")

generated := uuid.v4()
assert(uuid.isValid(generated))
assert(string.sub(generated, 15, 15) == "4")
raw := uuid.v4Raw()
assert(#raw == 32)

print("ok")
