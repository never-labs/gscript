print("case:sort_invalid_order_more2")

local function checkerror (msg, f, ...)
  local s, err = pcall(f, ...)
  assert(not s and string.find(err, msg))
end

local function check (t)
  local function f(a, b) assert(a and b); return true end
  checkerror("invalid order function", table.sort, t, f)
end

check{1,2,3,4}
check{1,2,3,4,5}
check{1,2,3,4,5,6}

print("ok")
