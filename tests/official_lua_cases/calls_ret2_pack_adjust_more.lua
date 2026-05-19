print("case:calls_ret2_pack_adjust_more")

local function unlpack(t, i)
  i = i or 1
  if i <= #t then
    return t[i], unlpack(t, i + 1)
  end
end

local function equaltab(t1, t2)
  assert(#t1 == #t2)
  for i = 1, #t1 do
    assert(t1[i] == t2[i])
  end
end

local function pack(...)
  return table.pack(...)
end

local a, b, c, d = unlpack({1, 2, 3})
assert(a == 1 and b == 2 and c == 3 and d == nil)

local t = {unlpack({1, 2, 3, 4})}
equaltab(t, {1, 2, 3, 4})

local p = pack(unlpack({1, 2, 3, 4}))
assert(p.n >= 1 and p[1] == 1)

print("ok")
