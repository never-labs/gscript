print("case:xpcall_iterator_edges_more")

local p = table.pack(xpcall(function () error("boom") end, function (e)
  assert(type(e) == "string")
  return nil, "ignored"
end))
assert(p.n == 2 and p[1] == false and p[3] == nil)

p = table.pack(xpcall(function () error("again") end, function (e)
  assert(type(e) == "string")
  return 1, 2, 3
end))
assert(p.n == 2 and p[1] == false and p[2] == 1 and p[3] == nil)

local function edge_iter(_, i)
  i = i + 1
  if i == 1 then
    return i, "one", "extra-one"
  end
  if i == 2 then
    return i, "two", "extra-two"
  end
  return nil, "stop-extra", "ignored"
end

local seen = 0
for i, label in edge_iter, nil, 0 do
  seen = seen + 1
  if seen == 1 then
    assert(i == 1 and label == "one")
  else
    assert(seen == 2 and i == 2 and label == "two")
  end
end
assert(seen == 2)

for i in edge_iter, nil, 0 do
  seen = seen + 10
  assert(i == 1 or i == 2)
end
assert(seen == 22)

print("ok")
