print("case:code_constant_branch_matrix_more2")

func choose(x) {
  if x == -2 { return "neg" }
  if x == 0 { return "zero" }
  if x == 3 { return "three" }
  if x == 42 { return "forty-two" }
  return "other"
}

seen := {}
for i := -3; i <= 43; i++ {
  seen[#seen + 1] = choose(i)
}

assert(seen[1] == "other")
assert(seen[2] == "neg")
assert(seen[4] == "zero")
assert(seen[7] == "three")
assert(seen[46] == "forty-two")
assert((2 ** 10) + (3 * 7) - 5 == 1040)

print("ok")
