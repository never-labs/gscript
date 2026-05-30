print("case:sort_invalid_order_function")

local function checkerror(msg, f, ...)
  local ok, err = pcall(f, ...)
  assert(not ok and type(err) == "string" and string.find(err, msg))
end

local a = setmetatable({}, {__len = function () return -1 end})
assert(#a == -1)
table.sort(a, error)

local function check(t)
  local function bad(a, b)
    assert(a and b)
    return true
  end
  checkerror("invalid order function", table.sort, t, bad)
end

check({1, 2, 3, 4})
check({1, 2, 3, 4, 5})
check({1, 2, 3, 4, 5, 6})

print("ok")
