print("case:closure_loop_mutation_more2")

local makers = {}
for i = 1, 5 do
  local base = i * 10
  makers[i] = function(delta)
    base = base + delta
    return base
  end
end

assert(makers[1](1) == 11)
assert(makers[1](4) == 15)
assert(makers[2](2) == 22)
assert(makers[5](-5) == 45)
assert(makers[2](3) == 25)

local function outer(x)
  local count = x
  return function()
    count = count + 1
    return function()
      return count
    end
  end
end

local nextfn = outer(7)
local a = nextfn()
local b = nextfn()
assert(a() == 9 and b() == 9)

print("ok")
