print("case:files_tmpname_remove_rename")

file := os.tmpname()
otherfile := os.tmpname()

f := io.open(file, "w")
assert(f)
assert(f:write("alo\n"))
assert(f:close())

assert(os.rename(file, otherfile))
assert(!os.rename(file, otherfile))

ok, msg := os.remove(otherfile)
assert(ok == true)
ok, msg = os.remove(otherfile)
assert(ok == nil && type(msg) == "string")

print("ok")
