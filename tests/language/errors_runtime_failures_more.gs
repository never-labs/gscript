print("case:errors_runtime_failures_more")

func checkerr(needle, f) {
  ok, err := pcall(f)
  assert(!ok && type(err) == "string")
}

func concat_function() { return print .. "a" }
func concat_bool() { return "a" .. false }
func concat_table() { return {} .. 2 }
func bad_collectgarbage() { collectgarbage("nooption") }
func yield_outside() { coroutine.yield() }

checkerr("concatenate", concat_function)
checkerr("concatenate", concat_bool)
checkerr("concatenate", concat_table)
checkerr("invalid option", bad_collectgarbage)
checkerr("yield", yield_outside)

a := {}
setmetatable(a, {__index: string})
func bad_self() { return a:sub() }

checkerr("bad argument", bad_self)

print("ok")
