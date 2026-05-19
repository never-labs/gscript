print("case:literals_string_table_more2")

words := {
  "alo",
  "alo\n",
  "\nalo\n",
  "",
  "brackets [] and equals ==",
}

assert(#words == 5)
assert(words[1] .. words[4] == "alo")
assert(string.sub(words[2], -1) == "\n")
assert(string.sub(words[3], 2, 4) == "alo")
assert(string.find(words[5], "[]", 1, true))
assert(string.find(words[5], "==", 1, true))

print("ok")
