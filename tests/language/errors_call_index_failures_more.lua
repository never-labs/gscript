print("case:errors_call_index_failures_more")

local function fails(f)
  local ok, err = pcall(f)
  assert(not ok and type(err) == "string")
end

fails(function() local a; return a(13) end)
fails(function() local a = {}; return a.bbbb(3) end)
fails(function() local aaa = {bbb = 1}; return aaa.bbb:ddd(9) end)
fails(function() local a, b, c; return (function() a = b + 1.1 end)() end)

print("ok")
