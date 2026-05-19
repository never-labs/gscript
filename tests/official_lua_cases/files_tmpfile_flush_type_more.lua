print("case:files_tmpfile_flush_type_more")

local f = assert(io.tmpfile())
assert(io.type(f) == "file")
assert(f:write("abc\n"))
assert(f:flush())
assert(f:seek("set", 0) == 0)
assert(f:read("a") == "abc\n")
assert(f:close())
assert(io.type(f) == "closed file")

print("ok")
