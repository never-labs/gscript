print("case:os_time_date_host_more")

now := os.time()
assert(type(now) == "number")
assert(now > 0)

clock1 := os.clock()
clock2 := os.clock()
assert(type(clock1) == "number")
assert(type(clock2) == "number")
assert(clock1 >= 0)
assert(clock2 >= 0)

assert(os.date("%Y", 1700000000) == "2023")
assert(os.date("year=%Y literal=%%", 1700000000) == "year=2023 literal=%")

host, hostErr := os.hostname()
assert((type(host) == "string" && host != "") || (host == nil && type(hostErr) == "string"))
assert(os.getpid() > 0)

args := os.args()
assert(type(args) == "table")
assert(type(args[1]) == "string")

print("ok")
