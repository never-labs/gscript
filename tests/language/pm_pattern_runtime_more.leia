print("case:pm_pattern_runtime_more")

words := {}
for w := range string.gmatch("skip alpha beta", "%w+", 6) {
  words[#words + 1] = w
}
assert(words[1] == "alpha" && words[2] == "beta" && words[3] == nil)

pos := {}
for p := range string.gmatch("ab cd", "()%w+", 4) {
  pos[#pos + 1] = p
}
assert(pos[1] == 4 && pos[2] == nil)

out, n := string.gsub("a=1 b=2 c=3", "(%w)=(%d)", func(k, v) {
  if k == "b" {
    return false
  }
  return k .. ":" .. (v + 10)
})
assert(out == "a:11 b=2 c:13" && n == 3)

out, n = string.gsub("one two three", "%w+", func(w) {
  if w == "two" {
    return nil
  }
  return "[" .. w .. "]"
})
assert(out == "[one] two [three]" && n == 3)

out, n = string.gsub("abcd", "().", func(i) {
  if i % 2 == 0 {
    return i
  }
  return nil
})
assert(out == "a2c4" && n == 4)

print("ok")
