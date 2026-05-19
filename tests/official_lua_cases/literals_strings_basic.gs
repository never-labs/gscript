print("case:literals_strings_basic")

assert("\n\"'\\" == "\n\"'\\")
assert("abcdefghi" == "abcdefghi")
assert("\n\t" == "\n\t")
assert(" [ " != " ] ")

b := "00123456789012345678901234567890123456789123456789012345678901234567890123456789"
assert(string.len(b) == 80)
assert("alo]\n]alo" == "alo]\n]alo")
assert(10 .. 20 .. -30 == "1020-30")

print("ok")
