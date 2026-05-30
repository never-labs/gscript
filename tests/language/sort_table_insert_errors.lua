print("case:sort_table_insert_errors")

local function checkerror (msg, f, ...)
  local s, err = pcall(f, ...)
  assert(not s and string.find(err, msg))
end

checkerror("wrong number of arguments", table.insert, {}, 2, 3, 4)
checkerror("bad argument", table.insert)
checkerror("table expected", table.insert, 1, 2)

print("ok")
