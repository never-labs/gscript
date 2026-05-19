print("case:errors_xpcall_nested_error_more")

local function f(x)
  if x == 0 then
    error("a\n")
  else
    local aux = function()
      return f(x - 1)
    end
    local a, b = xpcall(aux, aux)
    return a, b
  end
end

local ok, msg = f(3)
assert(ok and msg == true)

local res, handled = xpcall(function()
  error({code = 41})
end, function(m)
  return {code = m.code + 1}
end)

assert(not res and handled.code == 42)

print("ok")
