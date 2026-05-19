print("case:pm_match_minimal_more")

local function f(s, p)
  local i, e = string.find(s, p)
  if i then return string.sub(s, i, e) end
end

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
