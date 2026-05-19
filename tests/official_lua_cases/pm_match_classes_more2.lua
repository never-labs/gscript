print("case:pm_match_classes_more2")

local function f(s, p)
  local i, e = string.find(s, p)
  if i then return string.sub(s, i, e) end
end
assert(f("aloALO", "%l*") == "alo")
assert(f("aLo_ALO", "%a*") == "aLo")
assert(f("aaab", "a*") == "aaa")
assert(f("aaa", "ab*a") == "aa")
assert(f("aba", "ab*a") == "aba")
assert(f("aaab", "a+") == "aaa")
assert(not f("aaa", "b+"))
assert(not f("aaa", "ab+a"))
assert(f("aba", "ab+a") == "aba")
assert(f("a$a", ".%$") == "a$")
assert(f("a$a", ".$.") == "a$a")
assert(not f("a$b", "a$"))
assert(not f("aaa", "bb*"))

print("ok")
