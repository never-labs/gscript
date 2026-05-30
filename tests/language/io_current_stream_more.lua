print("case:io_current_stream_more")

local out_path = os.tmpname()
local out_file_path = os.tmpname()
local input_path = os.tmpname()
local current_lines_path = os.tmpname()
local lines_path = os.tmpname()

local f = assert(io.open(input_path, "w"))
assert(f:write("one\ntwo\nthree"))
assert(f:close())

f = assert(io.open(current_lines_path, "w"))
assert(f:write("alpha\nbeta\ngamma\n"))
assert(f:close())

f = assert(io.open(lines_path, "w"))
assert(f:write("red\ngreen\nblue\n"))
assert(f:close())

local old_out = io.output()
local path_out = assert(io.output(out_path))
assert(io.type(path_out) == "file")
io.write("path", "-", 12, "\n")
assert(io.flush())
assert(io.output(old_out) == old_out)
assert(path_out:close())

f = assert(io.open(out_path, "r"))
assert(f:read("a") == "path-12\n")
assert(f:close())

local explicit_out = assert(io.open(out_file_path, "w"))
old_out = io.output()
assert(io.output(explicit_out) == explicit_out)
io.write("file", "-", "handle", "\n")
assert(io.flush())
assert(io.output(old_out) == old_out)
assert(explicit_out:close())

f = assert(io.open(out_file_path, "r"))
assert(f:read("a") == "file-handle\n")
assert(f:close())

local old_in = io.input()
local path_in = assert(io.input(input_path))
assert(io.type(path_in) == "file")
local a, b, c = io.read("l", "L", "a")
assert(a == "one")
assert(b == "two\n")
assert(c == "three")
assert(io.input(old_in) == old_in)
assert(path_in:close())

local explicit_in = assert(io.open(current_lines_path, "r"))
old_in = io.input()
assert(io.input(explicit_in) == explicit_in)
assert(io.read() == "alpha")
local seen = {}
for line in io.lines() do
  seen[#seen + 1] = line
end
assert(#seen == 2 and seen[1] == "beta" and seen[2] == "gamma")
assert(io.input(old_in) == old_in)
assert(explicit_in:close())

local n = 0
local last = ""
for line in io.lines(lines_path) do
  n = n + 1
  last = line
end
assert(n == 3 and last == "blue")

local closed = assert(io.tmpfile())
assert(io.type(closed) == "file")
assert(closed:close())
assert(io.type(closed) == "closed file")
assert(io.type({}) == nil)

local ok = pcall(io.output, closed)
assert(not ok)
ok = pcall(io.input, closed)
assert(not ok)

assert(os.remove(out_path))
assert(os.remove(out_file_path))
assert(os.remove(input_path))
assert(os.remove(current_lines_path))
assert(os.remove(lines_path))

print("ok")
