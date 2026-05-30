print("case:calls_fixedpoint_returns")

local Z = function (le)
  local function a (f)
    return le(function (x) return f(f)(x) end)
  end
  return a(a)
end

local F = function (f)
  return function (n)
    if n == 0 then return 1
    else return n * f(n - 1) end
  end
end

local fat = Z(F)
assert(fat(0) == 1 and fat(4) == 24 and Z(F)(5) == 120)

local function g (z)
  local function f (a, b, c, d)
    return function (x, y) return a + b + c + d + a + x + y + z end
  end
  return f(z, z + 1, z + 2, z + 3)
end

local f = g(10)
assert(f(9, 16) == 10 + 11 + 12 + 13 + 10 + 9 + 16 + 10)

local function unlpack (t, i)
  i = i or 1
  if i <= #t then
    return t[i], unlpack(t, i + 1)
  end
end

local a, b, c, d = unlpack{1, 2, 3}
assert(a == 1 and b == 2 and c == 3 and d == nil)

print("ok")
