print("case:errors_error_edge_values")

ok, err := pcall(error)
assert(!ok && type(err) == "string" && string.find(err, "error"))

ok, err = pcall(error, "hi", 0)
assert(!ok && err == "hi")

func bad_add() {
  t := {}
  return t[#t] + 1
}

ok, err = pcall(bad_add)
assert(!ok && type(err) == "string")

ok, err = pcall(tonumber)
assert(!ok && type(err) == "string")

print("ok")
