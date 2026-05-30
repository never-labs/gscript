print("case:pm_gsub_error_subset_more")

local function checkerror(msg, f, ...)
  local s, err = pcall(f, ...)
  assert(not s and string.find(err, msg))
end

checkerror("invalid capture index %%2", string.gsub, "alo", ".", "%2")
checkerror("invalid use of '%%'", string.gsub, "alo", ".", "%x")

print("ok")
