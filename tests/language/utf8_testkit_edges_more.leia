print("case:utf8_testkit_edges_more")

pat := utf8.charpattern
assert(type(pat) == "string")
assert(#pat > 0)
assert(string.sub(pat, 1, 1) == "[")
assert(string.sub(pat, -1) == "*")

mixed := "A" .. utf8.char(0x4e2d) .. utf8.char(0x1f600) .. "z"
assert(utf8.sub(mixed, -2, -1) == utf8.char(0x1f600) .. "z")
assert(utf8.sub(mixed, -3, -2) == utf8.char(0x4e2d) .. utf8.char(0x1f600))
assert(utf8.sub(mixed, -100, 2) == "A" .. utf8.char(0x4e2d))
assert(utf8.sub(mixed, 3, 99) == utf8.char(0x1f600) .. "z")
assert(utf8.sub(mixed, 5, 99) == "")

assert(utf8.charclass(46) == "P")
assert(utf8.charclass(36) == "P")
assert(utf8.charclass(0) == "O")

before := testkit.snapshot()
after := testkit.memory()
delta := testkit.diff(before, after)
assert(type(delta.allocBytes) == "number")
assert(type(delta.allocKB) == "number")
assert(type(delta.sysBytes) == "number")
assert(type(delta.heapObjects) == "number")
assert(type(delta.numGC) == "number")
assert(type(delta.rootLog) == "number")
assert(type(delta.before) == "table")
assert(type(delta.after) == "table")

ok, report := testkit.checkMemory(before, {
  collect: true,
  maxAllocBytesGrowth: -1,
  maxHeapObjectsGrowth: -1,
  maxRootLogGrowth: -1,
})
assert(!ok && report.ok == false)
assert(type(report.allocBytes) == "number")
assert(type(report.heapObjects) == "number")
assert(type(report.rootLog) == "number")

numInfo := testkit.value(42)
floatInfo := testkit.value(1.5)
tableInfo := testkit.value({10, 20, 30})
fnInfoValue := nil
assert(numInfo.type == "number")
assert(type(numInfo.text) == "string")
assert(numInfo.truthy == true)
assert(type(numInfo.raw) == "string")
assert(type(numInfo.numberKind) == "string")
assert(floatInfo.type == "number")
assert(type(floatInfo.numberKind) == "string")
assert(tableInfo.type == "table")
assert(tableInfo.len == 3)
assert(tableInfo.truthy == true)

func pack(a, b, ...) {
  return a, b, select("#", ...)
}

fnInfoValue = testkit.value(pack)
assert(fnInfoValue.type == "function")
assert(fnInfoValue.truthy == true)
assert(type(fnInfoValue.text) == "string")
assert(type(fnInfoValue.raw) == "string")

assert(testkit.typeOf(nil) == "nil")
assert(testkit.typeOf(true) == "boolean")
assert(testkit.typeOf(1) == "number")
assert(testkit.typeOf("x") == "string")
assert(testkit.typeOf({}) == "table")
assert(testkit.typeOf(pack) == "function")

assert(testkit.equal("same", "same"))
assert(!testkit.equal({}, {}))
same := {}
assert(testkit.equal(same, same))

good := testkit.protect(pack, "a", "b", "c", "d")
assert(good.ok && good.n == 3)
assert(good.values[1] == "a" && good.values[2] == "b" && good.values[3] == 2)

func fail_string() {
  error("planned failure")
}

bad := testkit.protect(fail_string)
assert(!bad.ok)
assert(type(bad.error) == "string")
assert(string.find(bad.error, "planned failure", 1, true) != nil)

info := testkit.functionInfo(pack)
assert(info.type == "function")
assert(info.kind == "script")
assert(type(info.name) == "string")
assert(type(info.identity) == "string")
assert(type(info.raw) == "string")
assert(type(info.params) == "number")
assert(type(info.vararg) == "boolean")
assert(type(info.upvalues) == "number")

nativeInfo := testkit.functionInfo(print)
assert(nativeInfo.type == "function")
assert(nativeInfo.kind == "native")
assert(type(nativeInfo.name) == "string")
assert(type(nativeInfo.identity) == "string")
assert(type(nativeInfo.raw) == "string")

print("ok")
