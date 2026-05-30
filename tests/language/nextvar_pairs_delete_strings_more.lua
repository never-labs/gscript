print("case:nextvar_pairs_delete_strings_more")

local t = {}
for i = 1, 20 do t["k" .. i] = i end

local n = 0
for k, v in pairs(t) do
  n = n + 1
  assert(t[k] == v)
  t[k] = nil
end
assert(n == 20 and next(t) == nil)

print("ok")
