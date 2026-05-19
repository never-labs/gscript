print("case:files_file_lines_streams_more")

file := os.tmpname()
f := io.open(file, "w")
assert(f:write("alpha\nbeta\ngamma\n"))
assert(f:close())

f = io.open(file, "r")
iter := f:lines()
a := iter()
b := iter()
c := iter()
d := iter()
assert(a == "alpha" && b == "beta" && c == "gamma" && d == nil)
assert(io.type(f) == "file")
assert(f:close())
assert(io.type(f) == "closed file")

assert(io.type(io.stdin) == "file")
assert(io.type(io.stdout) == "file")
assert(io.type(io.stderr) == "file")

os.remove(file)

print("ok")
