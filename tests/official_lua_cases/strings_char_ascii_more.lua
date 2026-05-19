print("case:strings_char_ascii_more")

assert(string.char() == "")
assert(
  string.char(34) .. string.char(48) .. string.char(90) .. string.char(100) ==
  string.format("%1c%-c%-1c%c", 34, 48, 90, 100)
)
assert(string.byte(string.char(65)) == 65)
assert(string.char(65, 66, 67) == "ABC")

print("ok")
