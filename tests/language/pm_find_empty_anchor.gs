print("case:pm_find_empty_anchor")

a, b := string.find("", "")
assert(a == 1 && b == 0)

a, b = string.find("alo", "")
assert(a == 1 && b == 0)

assert(string.find("alo123alo", "12") == 4)
assert(!string.find("alo123alo", "^12"))
assert(string.find("abc", "^a") == 1)
assert(string.find("abc", "c$") == 3)

print("ok")
