print("case:nextvar_table_maxn")

function table.maxn (t)
  local max = 0
  for k in pairs(t) do
    max = (type(k) == "number") and math.max(max, k) or max
  end
  return max
end

assert(table.maxn{} == 0)
assert(table.maxn{["1000"] = true} == 0)
assert(table.maxn{["1000"] = true, [24.5] = 3} == 24.5)
assert(table.maxn{[1000] = true} == 1000)
assert(table.maxn{[10] = true, [100 * math.pi] = print} == 100 * math.pi)

table.maxn = nil
assert(table.maxn == nil)

print("ok")
