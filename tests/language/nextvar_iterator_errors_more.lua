print("case:nextvar_iterator_errors_more")

local function checkerror(msg, f, ...)
  local s, err = pcall(f, ...)
  assert(not s and string.find(err, msg))
end

assert(next{} == next{})
checkerror("invalid key", next, {10, 20}, 3)
checkerror("bad argument", pairs)
checkerror("bad argument", ipairs)
assert(next({}) == nil)
assert(next({}, nil) == nil)

print("ok")
