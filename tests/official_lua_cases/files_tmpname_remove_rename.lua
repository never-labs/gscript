print("case:files_tmpname_remove_rename")

local file = os.tmpname()
local otherfile = os.tmpname()

local f = assert(io.open(file, "w"))
assert(f:write("alo\n"))
assert(f:close())

assert(os.rename(file, otherfile))
assert(not os.rename(file, otherfile))

local ok, msg = os.remove(otherfile)
assert(ok == true)
ok, msg = os.remove(otherfile)
assert(ok == nil and type(msg) == "string")

print("ok")
