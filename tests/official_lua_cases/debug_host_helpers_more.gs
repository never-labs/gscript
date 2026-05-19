print("case:debug_host_helpers_more")

debugProbeValue := 42

func debugInner() {
  stack := debug.stack()
  trace := debug.traceback("boom")
  return stack, trace
}

func debugOuter() {
  return debugInner()
}

stack, trace := debugOuter()
assert(#stack >= 2)
assert(string.find(trace, "boom") != nil)
assert(string.find(trace, "debugInner") != nil)

globals := debug.globals()
assert(globals.debugProbeValue == 42)

info := debug.info(debugInner)
assert(info.kind == "script" && info.name == "debugInner")

value := debug.value({x: 1})
assert(value.type == "table" && value.truthy)

goStack := debug.goStack()
assert(string.find(goStack, "goroutine") != nil)

print("ok")
