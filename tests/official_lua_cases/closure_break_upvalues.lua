print("case:closure_break_upvalues")

local X, Y
local a = math.sin(0)

while a do
  local b = 10
  X = function () return b end
  if a then break end
end

do
  local b = 20
  Y = function () return b end
end

assert(X() == 10 and Y() == 20)

print("ok")
