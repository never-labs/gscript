print("case:nextvar_checknext_more")

local function checknext (a)
  local b = {}
  local k, v = next(a)
  while k do
    b[k] = v
    k, v = next(a, k)
  end
  for k,v in pairs(b) do assert(a[k] == v) end
  for k,v in pairs(a) do assert(b[k] == v) end
end

checknext{1,x=1,y=2,z=3}
checknext{1,2,x=1,y=2,z=3}
checknext{1,2,3,x=1,y=2,z=3}
checknext{1,2,3,4,x=1,y=2,z=3}
checknext{1,2,3,4,5,x=1,y=2,z=3}

print("ok")
