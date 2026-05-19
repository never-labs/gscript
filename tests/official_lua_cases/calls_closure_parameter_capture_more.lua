print("case:calls_closure_parameter_capture_more")

local function g (z)
  local function f (a,b,c,d)
    return function (x,y) return a+b+c+d+a+x+y+z end
  end
  return f(z,z+1,z+2,z+3)
end

local f = g(10)
assert(f(9, 16) == 10+11+12+13+10+9+16+10)

local function maker(a, b, c)
  return function(x)
    return a + b + c + x
  end
end

local h = maker(1, 2, 3, 4)
assert(h(10) == 16)
assert(not pcall(maker(5, 6), 7))

print("ok")
