print("case:strings_table_concat_empty_errors_more")

local function checkerror (msg, f, ...)
  local s, err = pcall(f, ...)
  assert(not s and string.find(err, msg))
end

checkerror("table expected", table.concat, 3)
assert(table.concat{} == "")
assert(table.concat({}, "x") == "")
assert(table.concat({}, "x", 10, 9) == "")
assert(table.concat({}, "x", -9, -10) == "")
assert(not pcall(table.concat, {"a", "b", {}}))

print("ok")
