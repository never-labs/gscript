print("case:errors_runtime_messages_more")

local function fails(f)
  local ok, err = pcall(f)
  assert(not ok and type(err) == "string")
end

fails(function() return {} + 1 end)
fails(function() local a; return a(13) end)
fails(function() local a = {}; return a.bbbb(3) end)
fails(function() return #3 end)
fails(function() return #print end)
fails(function() local a, b, c; return (a and b or c)() end)
fails(function() return print < 10 end)
fails(function() return print < print end)
fails(function() return "10" < 10 end)
fails(function() return 10 < "23" end)

print("ok")
