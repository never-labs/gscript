print("case:strings_reverse_ascii_more")

assert(string.reverse("") == "")
assert(string.reverse("abcd") == "dcba")
assert(string.reverse("123456789") == "987654321")
assert(string.reverse(string.rep("ab", 4)) == "babababa")

print("ok")
