print("case:goto_if_branch_equivalent_more")

func testG(a) {
  if a == 1 { return "1" }
  if a == 2 { return "2" }
  if a == 3 { return "3" }
  if a == 4 { return a + 1 }
  return a * 2
}

assert(testG(1) == "1")
assert(testG(2) == "2")
assert(testG(3) == "3")
assert(testG(4) == 5)
assert(testG(5) == 10)

print("ok")
