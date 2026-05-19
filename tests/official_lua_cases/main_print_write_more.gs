print("case:main_print_write_more")

io.write("alpha")
io.write("-", 12, "\n")
print("beta", "gamma")
assert(type(print) == "function")
assert(type(io.write) == "function")

print("ok")
