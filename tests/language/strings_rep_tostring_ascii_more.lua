print("case:strings_rep_tostring_ascii_more")

assert(string.upper("ab c") == "AB C")
assert(string.lower("ABCc%$") == "abcc%$")
assert(string.rep("teste", 0) == "")
assert(string.rep("", 10) == "")
assert(string.rep("teste", 0, "xuxu") == "")
assert(string.rep("teste", 1, "xuxu") == "teste")
assert(string.rep("", 10, ".") == string.rep(".", 9))
assert(string.reverse("") == "")

for i = 0, 30 do
  assert(string.len(string.rep("a", i)) == i)
end

assert(type(tostring(nil)) == "string")
assert(type(tostring(12)) == "string")
assert(tostring(true) == "true")
assert(tostring(false) == "false")
assert(tostring(-1203) == "-1203")
assert(tostring(1203.125) == "1203.125")
assert(tostring(-0.5) == "-0.5")

print("ok")
