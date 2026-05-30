print("case:calls_inline_function_args")

ok, got = pcall(function(x) return x + 7 end, 5)
assert(ok and got == 12)

seen = 0
values = {4, 1, 3, 2}
table.sort(values, function(a, b)
  seen = seen + 1
  return a > b
end)
assert(seen > 0)
assert(table.concat(values, ",") == "4,3,2,1")

function f(cb, x)
  return cb(x), cb(x + 1)
end
a, b = f(function(n) return n * 3 end, 6)
assert(a == 18 and b == 21)

print("ok")
