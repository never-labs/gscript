print("case:db_vm_debug_parity_more")

events := {}
sawCall := false
sawReturn := false
sawEmit := false
sawError := false
sawSink := false

func hookFn(e) {
  table.insert(events, e.type .. ":" .. e.kind .. ":" .. e.name)
  if e.type == "call" {
    sawCall = true
  }
  if e.type == "return" {
    sawReturn = true
  }
  if e.type == "emit" {
    sawEmit = true
  }
  if e.type == "error" {
    sawError = true
  }
}

func sinkFn(e) {
  table.insert(events, "sink:" .. e.event)
  sawSink = true
}

func leaf() {
  stack := debug.stack()
  i0 := debug.info(0)
  i1 := debug.info(1)
  debug.emit("mark", {ok: true})
  return stack[#stack].name, i0.name, i1.name, i0.sourceName, i0.line, i0.column
}

debug.setHook(hookFn)
debug.setSink(sinkFn)

func parent() {
  return leaf()
}

func fail() {
  error("debug-fail")
}

topName, info0Name, info1Name, sourceName, lineNumber, colNumber := parent()
pcall(fail)
debug.setHook(nil)
debug.setSink(nil)

assert(topName == "leaf")
assert(info0Name == "leaf")
assert(info1Name == "parent")
assert(string.find(sourceName, "db_vm_debug_parity_more.gs", 1, true) != nil)
assert(lineNumber > 0)
assert(colNumber == 1)
assert(#events > 0)
assert(sawCall)
assert(sawReturn)
assert(sawEmit)
assert(sawError)
assert(sawSink)

print("ok")
