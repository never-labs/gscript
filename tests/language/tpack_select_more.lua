print("case:tpack_select_more")

local function check(...)
  assert(select("#", ...) == 5)
  assert(select(1, ...) == "a")
  assert(select(2, ...) == nil)
  assert(select(-1, ...) == "e")
  assert(select(-2, ...) == 4)
  local a, b, c = select(3, ...)
  assert(a == 3 and b == 4 and c == "e")
end

check("a", nil, 3, 4, "e")

print("ok")
