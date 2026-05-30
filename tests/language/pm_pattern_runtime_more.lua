print("case:pm_pattern_runtime_more")

local words = {}
for w in string.gmatch("skip alpha beta", "%w+", 6) do
  words[#words + 1] = w
end
assert(words[1] == "alpha" and words[2] == "beta" and words[3] == nil)

local pos = {}
for p in string.gmatch("ab cd", "()%w+", 4) do
  pos[#pos + 1] = p
end
assert(pos[1] == 4 and pos[2] == nil)

local out, n = string.gsub("a=1 b=2 c=3", "(%w)=(%d)", function(k, v)
  if k == "b" then
    return false
  end
  return k .. ":" .. (v + 10)
end)
assert(out == "a:11 b=2 c:13" and n == 3)

out, n = string.gsub("one two three", "%w+", function(w)
  if w == "two" then
    return nil
  end
  return "[" .. w .. "]"
end)
assert(out == "[one] two [three]" and n == 3)

out, n = string.gsub("abcd", "().", function(i)
  if i % 2 == 0 then
    return i
  end
  return nil
end)
assert(out == "a2c4" and n == 4)

print("ok")
