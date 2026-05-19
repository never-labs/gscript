print("case:nextvar_pairs_many_string_keys")

local a = {}
for i = 0, 10000 do
  if math.fmod(i, 10) ~= 0 then
    a["x" .. i] = i
  end
end

local n = {n = 0}
for k, v in pairs(a) do
  n.n = n.n + 1
  assert(k and v and a[k] == v)
end

assert(n.n == 9000)

print("ok")
