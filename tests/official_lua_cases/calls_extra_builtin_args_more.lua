print("case:calls_extra_builtin_args_more")

rawget({}, "x", 1)
rawset({}, "x", 1, 2)
assert(math.sin(1, 2) == math.sin(1))

local t = {10, 9, 8, 4, 19, 23, 0, 0}
table.sort(t, function(a, b) return a < b end, "extra arg")
for i = 2, #t do
  assert(t[i - 1] <= t[i])
end

print("ok")
