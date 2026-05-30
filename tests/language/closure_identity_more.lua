print("case:closure_identity_more")

local a = {}
for i = 1, 5 do
  a[i] = function(x) return i end
end
assert(a[3] ~= a[4] and a[4] ~= a[5])

do
  local a = function(x) return math.sin(x) end
  local function f()
    return a
  end
  assert(f() == f())
end

print("ok")
