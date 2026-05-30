print("case:nextvar_generic_for_multivalue_more")

local function f(n, p)
  local t = {}
  for i = 1, p do
    t[i] = i * 10
  end
  return function(_, n, ...)
    assert(select("#", ...) == 0)
    if n > 0 then
      n = n - 1
      return n, table.unpack(t)
    end
  end, nil, n
end

local x = 0
for n, a in f(5, 3) do
  x = x + 1
  assert(a == 10)
end
assert(x == 5)

print("ok")
