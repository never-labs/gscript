print("case:closure_if_branch_upvalues_more")

local a = {}
for i = 1, 10 do
  if i % 3 == 0 then
    local y = 0
    a[i] = function(x) local t = y; y = x; return t end
  elseif i % 3 == 1 then
    local y = 1
    a[i] = function(x) local t = y; y = x; return t end
  elseif i % 3 == 2 then
    local y = 2
    a[i] = function(x) local t = y; y = x; return t end
  end
end

for i = 1, 10 do
  assert(a[i](i * 10) == i % 3 and a[i]() == i * 10)
end

print("ok")
