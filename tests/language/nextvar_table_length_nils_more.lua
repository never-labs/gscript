print("case:nextvar_table_length_nils_more")

assert(#{} == 0)
assert(#{nil} == 0)
assert(#{nil, nil} == 0)
assert(#{nil, nil, nil} == 0)
assert(#{nil, nil, nil, nil} == 0)
assert(#{1, 2, 3, nil, nil} == 3)

a = {}
for i = 1, 100 do
  a[i] = true
  assert(#a == i)
end
for i = 5, 95 do a[i] = nil end
for i = 1, 4 do assert(a[i] == true) end
for i = 5, 95 do assert(a[i] == nil) end
for i = 96, 100 do assert(a[i] == true) end

print("ok")
