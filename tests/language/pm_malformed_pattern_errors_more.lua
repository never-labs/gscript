print("case:pm_malformed_pattern_errors_more")

local function malform(p)
  local r = pcall(string.find, "a", p)
  assert(not r)
end

malform("(.")
malform(".)")
malform("[a")
malform("[]")
malform("[^]")
malform("[a%]")
malform("[a%")

print("ok")
