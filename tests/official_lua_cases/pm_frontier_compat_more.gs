print("case:pm_frontier_compat_more")

f := func(s, p) {
  i, e := string.find(s, p)
  if i { return string.sub(s, i, e), i, e }
}

assert(f("abc def", "%f[%w]%w+") == "abc")
assert(f("abc,def", "%w+%f[%W]") == "abc")

word, a, b := f("..hello,42", "%f[%a]%a+%f[%W]")
assert(word == "hello" && a == 3 && b == 7)

digits, di, de := f("abc123 def", "%f[%d]%d+")
assert(digits == "123" && di == 4 && de == 6)

zs, ze := string.find("abc", "%f[%z]")
assert(zs == 4 && ze == 3)

out, n := string.gsub("one two", "%f[%w](%w+)", "[%1]")
assert(out == "[one] [two]" && n == 2)

doubled, doubledN := string.gsub("a b", "%f[%w]%w", "%1%0")
assert(doubled == "aa bb" && doubledN == 2)

t := {}
for w := range string.gmatch("a-b c", "%f[%w]%w+%f[%W]") {
  table.insert(t, w)
}
assert(table.concat(t, "|") == "a|b|c")

print("ok")
