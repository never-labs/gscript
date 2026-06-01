print("case:regexp_go_host_more")

assert(regexp.match("^hello", "hello world") == true)
assert(regexp.find("[0-9]+", "abc123def") == "123")

all := regexp.findAll("[0-9]+", "a1b22c333")
assert(#all == 3)
assert(all[1] == "1")
assert(all[3] == "333")

assert(regexp.replace("[0-9]+", "a1b22c333", "X") == "aXb22c333")
assert(regexp.replaceAll("[0-9]+", "a1b22c333", "X") == "aXbXcX")

parts := regexp.split(",\\s*", "a, b, c")
assert(#parts == 3)
assert(parts[2] == "b")

re, err := regexp.compile("([a-z]+)([0-9]+)")
assert(err == nil)
assert(re.pattern == "([a-z]+)([0-9]+)")
sub := re.findSubmatch("abc123")
assert(sub[1] == "abc123")
assert(sub[2] == "abc")
assert(sub[3] == "123")
assert(re.numSubexp() == 2)
assert(re.replaceAll("a1 b22", "N") == "N N")

bad, err := regexp.compile("[")
assert(bad == nil)
assert(type(err) == "string")

ok, err := pcall(regexp.mustCompile, "[")
assert(ok == false)
assert(type(err) == "string")

print("ok")
