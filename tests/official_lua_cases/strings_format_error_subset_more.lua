print("case:strings_format_error_subset_more")

local function checkerror(msg, f, ...)
  local s, err = pcall(f, ...)
  assert(not s and string.find(err, msg))
end

checkerror("invalid", string.format, "%t", 10)
checkerror("no value", string.format, "%d %d", 10)
checkerror("invalid", string.format, "%F", 10)

print("ok")
