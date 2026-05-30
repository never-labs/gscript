print("case:db_pcall_nested_errors")

local function bad_add()
  return "joao" + 1
end

local function outer(n)
  if n == 0 then
    return bad_add()
  end
  return outer(n - 1)
end

local ok, msg = pcall(outer, 3)
assert(not ok and type(msg) == "string")

local function guards(f)
  local ok1 = pcall(f)
  local ok2, val = pcall(function () return pcall(f) end)
  return ok1, ok2, val
end

local a, b, c = guards(bad_add)
assert(a == false and b == true and c == false)

print("ok")
