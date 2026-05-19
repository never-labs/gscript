print("case:locals_many_shadow_slots_more2")

local total = 0
for i = 1, 12 do
  local a, b, c = i, i + 1, i + 2
  do
    local a, d = b + c, i * 2
    total = total + a + d
  end
  total = total + a + b + c
end

assert(total == 618)

local function pack(a, b, c, d)
  local b = b or 0
  local d = d or 0
  return a + b + c + d
end

assert(pack(1, nil, 3, nil) == 4)
assert(pack(4, 5, 6, 7) == 22)

print("ok")
