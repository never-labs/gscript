print("case:calls_incorrect_args_more")

rawget({}, "x", 1)
rawset({}, "x", 1, 2)
assert(math.sin(1, 2) == math.sin(1))
local a = {10, 9, 8, 4, 19, 23, 0, 0}
table.sort(a, function(x, y) return x < y end, "extra arg")
for i = #a, 2, -1 do assert(not (a[i] < a[i - 1])) end

assert((function () return nil end)(4) == nil)
assert((function () local a; return a end)(4) == nil)
assert((function (a) return a end)() == nil)

print("ok")
