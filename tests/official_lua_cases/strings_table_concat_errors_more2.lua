print("case:strings_table_concat_errors_more2")

local function checkerror(msg, f, ...)
  local s, err = pcall(f, ...)
  assert(not s and string.find(err, msg))
end
checkerror("table expected", table.concat, 3)
assert(not pcall(table.concat, {"a", "b", {}}))

print("ok")
