print("case:literals_line_comment_strings_more")

local a = 1        -- a comment
local b = 2


local x = "hi\n"
local y = "\nhello\r\n\n"
assert(a + b == 3)
assert(x == "hi\n")
assert(string.len(y) == 9)
assert(string.sub(y, 2, 6) == "hello")

print("ok")
