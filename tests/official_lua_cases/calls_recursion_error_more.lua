print("case:calls_recursion_error_more")

local function err_on_n(n)
  if n == 0 then error("boom")
  else err_on_n(n - 1)
  end
end

local function dummy(n)
  if n > 0 then
    assert(not pcall(err_on_n, n))
    dummy(n - 1)
  end
end

dummy(10)

local function deep(n)
  if n > 0 then return deep(n - 1) end
  return 101
end
assert(deep(500) == 101)

print("ok")
