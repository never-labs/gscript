print("case:files_append_read_all_more")

local file = os.tmpname()

local f = assert(io.open(file, "w"))
assert(f:write("first\n"))
assert(f:close())

f = assert(io.open(file, "a"))
assert(f:write("second\n"))
assert(f:close())

f = assert(io.open(file, "r"))
local all = f:read("*a")
assert(all == "first\nsecond\n")
assert(f:close())

local count = 0
for line in io.lines(file) do
  count = count + 1
  if count == 1 then assert(line == "first") end
  if count == 2 then assert(line == "second") end
end
assert(count == 2)

assert(os.remove(file))

print("ok")
