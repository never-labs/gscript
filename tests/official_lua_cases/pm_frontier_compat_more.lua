print("case:pm_frontier_compat_more")

local function f(s, p)
  local i, e = string.find(s, p)
  if i then return string.sub(s, i, e), i, e end
end

assert(f("abc def", "%f[%w]%w+") == "abc")
assert(f("abc,def", "%w+%f[%W]") == "abc")

local word, a, b = f("..hello,42", "%f[%a]%a+%f[%W]")
assert(word == "hello" and a == 3 and b == 7)

local digits, di, de = f("abc123 def", "%f[%d]%d+")
assert(digits == "123" and di == 4 and de == 6)

local zs, ze = string.find("abc", "%f[%z]")
assert(zs == 4 and ze == 3)

local out, n = string.gsub("one two", "%f[%w](%w+)", "[%1]")
assert(out == "[one] [two]" and n == 2)

local doubled, doubledN = string.gsub("a b", "%f[%w]%w", "%1%0")
assert(doubled == "aa bb" and doubledN == 2)

local t = {}
for w in string.gmatch("a-b c", "%f[%w]%w+%f[%W]") do
  t[#t + 1] = w
end
assert(table.concat(t, "|") == "a|b|c")

print("ok")
