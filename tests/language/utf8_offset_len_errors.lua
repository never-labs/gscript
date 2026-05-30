print("case:utf8_offset_len_errors")

local function checkerror (msg, f, ...)
  local s, err = pcall(f, ...)
  assert(not s and string.find(err, msg))
end

checkerror("position out of bounds", utf8.offset, "abc", 1, 5)
checkerror("position out of bounds", utf8.offset, "abc", 1, -4)
checkerror("position out of bounds", utf8.offset, "", 1, 2)
checkerror("position out of bounds", utf8.offset, "", 1, -1)
checkerror("continuation byte", utf8.offset, "𦧺", 1, 2)
checkerror("out of bounds", utf8.len, "abc", 0, 2)
checkerror("out of bounds", utf8.len, "abc", 1, 4)

assert(utf8.len("abc", 1, 3) == 3)
assert(utf8.len("abc", 2, 2) == 1)

print("ok")
