print("case:utf8_char_range_errors")

local function checkerror (msg, f, ...)
  local s, err = pcall(f, ...)
  assert(not s and string.find(err, msg))
end

assert(utf8.char() == "")
assert(utf8.char(97, 98, 99) == "abc")

checkerror("value out of range", utf8.char, 0x7FFFFFFF + 1)
checkerror("value out of range", utf8.char, -1)

print("ok")
