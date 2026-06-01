print("case:tpack_select_more")

func check(...) {
  assert(select("#", ...) == 5)
  assert(select(1, ...) == "a")
  assert(select(2, ...) == nil)
  assert(select(-1, ...) == "e")
  assert(select(-2, ...) == 4)
  a, b, c := select(3, ...)
  assert(a == 3 && b == 4 && c == "e")
}

check("a", nil, 3, 4, "e")

print("ok")
