print("case:utf8_basic")

assert(!utf8.offset("alo", 5))
assert(!utf8.offset("alo", -4))

s := "hello World"
for i := 1; i <= utf8.len(s); i++ {
    assert(string.byte(s, i) == string.byte(s, i))
}

assert(utf8.char() == "")
assert(utf8.char(97, 98, 99) == "abc")
assert(utf8.codepoint("abc", 1, 1) == 97)
assert(utf8.offset("alo", 2) == 2)
assert(utf8.len("汉字/漢字") == 5)

print("ok")
