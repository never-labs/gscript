print("case:strings_rep_reverse_tostring")

assert(string.rep("teste", 0) == "")
assert(string.rep("teste", 1, "xuxu") == "teste")
assert(string.rep("teste", 3, "x") == "testextestexteste")
assert(string.rep("", 10, ".") == string.rep(".", 9))

binary := string.char(1, 0, 1)
assert(string.rep(binary, 2, string.char(0, 0)) ==
       string.char(1, 0, 1, 0, 0, 1, 0, 1))

assert(string.reverse("") == "")
assert(string.reverse(string.char(0, 1, 2, 3)) == string.char(3, 2, 1, 0))
assert(string.reverse(string.char(0) .. "1234") == "4321" .. string.char(0))

for i := 0; i <= 30; i = i + 1 {
  assert(string.len(string.rep("a", i)) == i)
}

assert(type(tostring(nil)) == "string")
assert(type(tostring(12)) == "string")
assert(string.find(tostring({}), "table:"))
assert(string.find(tostring(print), "function:"))
assert(#tostring(string.char(0)) == 1)
assert(tostring(true) == "true")
assert(tostring(false) == "false")
assert(tostring(-1203) == "-1203")
assert(tostring(1203.125) == "1203.125")
assert(tostring(-0.5) == "-0.5")

print("ok")
