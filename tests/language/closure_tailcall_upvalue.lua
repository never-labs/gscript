print("case:closure_tailcall_upvalue")

local function t ()
  local function c (a, b)
    assert(a == "test" and b == "OK")
  end
  local function v (f, ...)
    c("test", f() ~= 1 and "FAILED" or "OK")
  end
  local x = 1
  return v(function () return x end)
end

t()

print("ok")
