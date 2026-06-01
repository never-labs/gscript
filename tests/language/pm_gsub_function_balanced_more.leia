print("case:pm_gsub_function_balanced_more")

func isbalanced(s) {
  return string.find(string.gsub(s, "%b()", ""), "[()]") == nil
}

assert(isbalanced("(9 ((8))(\0) 7) \0\0 a b ()(c)() a"))
assert(!isbalanced("(9 ((8) 7) a b (\0 c) a"))
assert(string.gsub("alo 'oi' alo", "%b''", "\"") == "alo \" alo")

a, b := string.gsub("um (dois) tres (quatro)", "(%(%w+%))", "\"")
assert(a == "um \" tres \"" && b == 2)

print("ok")
