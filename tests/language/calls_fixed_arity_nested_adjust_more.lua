print("case:calls_fixed_arity_nested_adjust_more")

function triple()
  return 10, 20, 30
end

function id(x)
  return x
end

function join3(a, b, c)
  return a, b, c
end

a, b, c = join3(id(triple()), id((triple())), 99)
assert(a == 10 and b == 10 and c == 99)

function F(n)
  if n == 0 then return 1 end
  return n - M(F(n - 1))
end

function M(n)
  if n == 0 then return 0 end
  return n - F(M(n - 1))
end

assert(F(8) == 5)
assert(M(8) == 5)
assert(F(16) == 10)

print("ok")
