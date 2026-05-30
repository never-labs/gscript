print("case:pm_match_minimal_more")

func f(s, p) {
  i, e := string.find(s, p)
  if i { return string.sub(s, i, e) }
}

assert(f("aaab", "a-") == "")
assert(f("aaa", "^.-$") == "aaa")
assert(f("aabaaabaaabaaaba", "b.*b") == "baaabaaabaaab")
assert(f("aabaaabaaabaaaba", "b.-b") == "baaab")
assert(f("alo xo", ".o$") == "xo")
assert(f(" \n isto e assim", "%S%S*") == "isto")
assert(f(" \n isto e assim", "%S*$") == "assim")
assert(f(" \n isto e assim", "[a-z]*$") == "assim")
assert(f("um caracter ? extra", "[^%sa-z]") == "?")
assert(f("", "a?") == "")
assert(f("aa", "^aa?a?a") == "aa")
assert(f("]]]ab", "[^]]+") == "ab")

print("ok")
