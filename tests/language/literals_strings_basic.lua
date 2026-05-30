print("case:literals_strings_basic")

assert("\n\"'\\" == [[

"'\]])
assert("abc\z
        def\z
        ghi\z
       " == "abcdefghi")
assert("\n\t" == [[

	]])
assert([[ [ ]] ~= [[ ] ]])

local b = "00123456789012345678901234567890123456789123456789012345678901234567890123456789"
assert(string.len(b) == 80)
assert("alo]\n]alo" == [[alo]
]alo]])
assert(010 .. 020 .. -030 == "1020-30")

print("ok")
