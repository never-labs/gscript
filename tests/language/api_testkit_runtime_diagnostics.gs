print("case:api_testkit_runtime_diagnostics")

before := testkit.snapshot()
collectgarbage("collect")
ok, report := testkit.checkMemory(before, {collect: true})
assert(ok && report.ok)
assert(type(testkit.memory().allocKB) == "number")
assert(type(testkit.diff(before).numGC) == "number")

func add(a, b) {
  return a + b, "ok"
}

func fail() {
  error({code: "boom"})
}

good := testkit.protect(add, 2, 5)
assert(good.ok && good.n == 2 && good.values[1] == 7 && good.values[2] == "ok")

bad := testkit.protect(fail)
assert(!bad.ok && type(bad.error) == "table" && bad.error.code == "boom")

assert(testkit.typeOf({}) == "table")
assert(testkit.value("abc").len == 3)
assert(testkit.value(false).truthy == false)
assert(testkit.equal(add, add))
assert(testkit.sameFunction(add, add))
assert(!testkit.sameFunction(add, print))

info := testkit.functionInfo(add)
assert(type(info.identity) == "string")
assert(info.kind == "script")

print("ok")
