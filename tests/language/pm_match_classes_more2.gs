print("case:pm_match_classes_more2")

func f(s, p) {
  i, e := string.find(s, p)
  if i { return string.sub(s, i, e) }
}
assert(f("aloALO", "%l*") == "alo")
assert(f("aLo_ALO", "%a*") == "aLo")
assert(f("aaab", "a*") == "aaa")
assert(f("aaa", "ab*a") == "aa")
assert(f("aba", "ab*a") == "aba")
assert(f("aaab", "a+") == "aaa")
assert(!f("aaa", "b+"))
assert(!f("aaa", "ab+a"))
assert(f("aba", "ab+a") == "aba")
assert(f("a$a", ".%$") == "a$")
assert(f("a$a", ".$.") == "a$a")
assert(!f("a$b", "a$"))
assert(!f("aaa", "bb*"))

print("ok")
