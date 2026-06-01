print("case:main_generated_chunk_eval_more")

fn := script.compile("return base + 2, name", script.env({name: "generated", base: 10}))
a, b := fn()
assert(a == 12 && b == "generated")
assert(type(fn) == "function")

ok, err := pcall(script.compile, "return !", script.env({name: "generated"}))
assert(!ok && type(err) == "string")

print("ok")
