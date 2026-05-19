print("case:nextvar_length_more")

assert(#{} == 0)
assert(#{nil} == 0)
assert(#{nil, nil} == 0)
assert(#{nil, nil, nil} == 0)
assert(#{nil, nil, nil, nil} == 0)
assert(#{1, 2, 3, nil, nil} == 3)
assert(#{[-1] = 2} == 0)

for i = 0, 40 do
  local a = {}
  for j = 1, i do
    a[j] = j
  end
  assert(#a == i)
end

print("ok")
