print("case:files_seek_overwrite_more")

local file = os.tmpname()

local f = assert(io.open(file, "w"))
assert(f:seek() == 0)
assert(f:write("abcdef"))
assert(f:seek() == 6)
assert(f:seek("set", 2) == 2)
assert(f:write("XY"))
assert(f:seek() == 4)
assert(f:close())

f = assert(io.open(file, "r"))
assert(f:read("a") == "abXYef")
assert(f:close())
assert(os.remove(file))

print("ok")
