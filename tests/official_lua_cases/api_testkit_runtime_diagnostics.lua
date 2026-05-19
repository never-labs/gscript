print("case:api_testkit_runtime_diagnostics")

local testkit = {}

function testkit.snapshot()
  return {
    allocBytes = 0,
    allocKB = collectgarbage("count"),
    sysBytes = 0,
    heapObjects = 0,
    numGC = 0,
    rootLog = 0,
    running = true,
    mode = "oracle",
  }
end

testkit.memory = testkit.snapshot

function testkit.diff(before, after)
  after = after or testkit.snapshot()
  return {
    allocBytes = after.allocBytes - before.allocBytes,
    allocKB = after.allocKB - before.allocKB,
    sysBytes = after.sysBytes - before.sysBytes,
    heapObjects = after.heapObjects - before.heapObjects,
    numGC = after.numGC - before.numGC,
    rootLog = after.rootLog - before.rootLog,
  }
end

function testkit.checkMemory(before, opts)
  local report = testkit.diff(before, testkit.snapshot())
  report.ok = true
  return true, report
end

function testkit.value(v)
  local out = {type = type(v), text = tostring(v), truthy = not not v}
  if type(v) == "number" then
    out.numberKind = "number"
  elseif type(v) == "string" or type(v) == "table" then
    out.len = #v
  elseif type(v) == "function" then
    out.functionKind = "function"
    out.identity = tostring(v)
  end
  return out
end

function testkit.typeOf(v) return type(v) end
function testkit.equal(a, b) return a == b end
function testkit.sameFunction(a, b) return type(a) == "function" and type(b) == "function" and a == b end

function testkit.functionInfo(fn)
  return {type = "function", kind = "function", name = "oracle", identity = tostring(fn)}
end

function testkit.protect(fn, ...)
  local values = table.pack(pcall(fn, ...))
  if not values[1] then
    return {ok = false, error = values[2]}
  end
  local out = {ok = true, values = {}, n = values.n - 1}
  for i = 2, values.n do out.values[i - 1] = values[i] end
  return out
end

local before = testkit.snapshot()
collectgarbage()
local ok, report = testkit.checkMemory(before, {collect = true})
assert(ok and report.ok)
assert(type(testkit.memory().allocKB) == "number")
assert(type(testkit.diff(before).numGC) == "number")

local function add(a, b) return a + b, "ok" end
local function fail() error({code = "boom"}) end

local good = testkit.protect(add, 2, 5)
assert(good.ok and good.n == 2 and good.values[1] == 7 and good.values[2] == "ok")

local bad = testkit.protect(fail)
assert(not bad.ok and type(bad.error) == "table" and bad.error.code == "boom")

assert(testkit.typeOf({}) == "table")
assert(testkit.value("abc").len == 3)
assert(testkit.value(false).truthy == false)
assert(testkit.equal(add, add))
assert(testkit.sameFunction(add, add))
assert(not testkit.sameFunction(add, print))

local info = testkit.functionInfo(add)
assert(type(info.identity) == "string")

print("ok")
