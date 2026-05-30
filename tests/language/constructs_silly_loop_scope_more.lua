print("case:constructs_silly_loop_scope_more")

repeat until 1; repeat until true
while false do end; while nil do end

do
  local a
  local function shadow(x)
    x = {a=1}
    x = {x=1}
    x = {G=1}
    return a, x.G
  end
  local aa, gg = shadow({})
  assert(aa == nil and gg == 1)
end

local function f (i)
  while 1 do
    if i>0 then i=i-1
    else return end
  end
end

local function g(i)
  while 1 do
    if i>0 then i=i-1
    else return end
  end
end

f(10); g(10)

print("ok")
