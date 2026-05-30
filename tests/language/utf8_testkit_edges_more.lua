print("case:utf8_testkit_edges_more")

local function utf8_sub(s, i, j)
  local runes = {}
  for _, cp in utf8.codes(s) do
    runes[#runes + 1] = utf8.char(cp)
  end
  j = j or #runes
  if i < 0 then i = #runes + i + 1 end
  if j < 0 then j = #runes + j + 1 end
  if i < 1 then i = 1 end
  if j > #runes then j = #runes end
  if i > j + 1 then return "" end
  local out = {}
  for n = i, j do out[#out + 1] = runes[n] end
  return table.concat(out)
end

function utf8.charclass(cp)
  if (cp >= 65 and cp <= 90) or (cp >= 97 and cp <= 122) then
    return "L"
  elseif cp >= 48 and cp <= 57 then
    return "N"
  elseif cp == 32 or cp == 9 or cp == 10 or cp == 13 then
    return "S"
  elseif cp == 46 or cp == 36 then
    return "P"
  end
  return "O"
end

utf8.sub = utf8_sub

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
    before = before,
    after = after,
  }
end

function testkit.checkMemory(before, opts)
  if opts and opts.collect then collectgarbage() end
  local report = testkit.diff(before, testkit.snapshot())
  local ok = true
  if opts and opts.maxAllocBytesGrowth and report.allocBytes > opts.maxAllocBytesGrowth then ok = false end
  if opts and opts.maxHeapObjectsGrowth and report.heapObjects > opts.maxHeapObjectsGrowth then ok = false end
  if opts and opts.maxRootLogGrowth and report.rootLog > opts.maxRootLogGrowth then ok = false end
  report.ok = ok
  return ok, report
end

function testkit.value(v)
  local out = {type = type(v), text = tostring(v), truthy = not not v, raw = tostring(v)}
  if type(v) == "number" then
    out.numberKind = "number"
  elseif type(v) == "string" or type(v) == "table" then
    out.len = #v
  elseif type(v) == "function" then
    out.functionKind = "script"
    out.identity = tostring(v)
    out.name = "oracle"
  end
  return out
end

function testkit.typeOf(v) return type(v) end
function testkit.equal(a, b) return a == b end

function testkit.functionInfo(fn)
  return {
    type = "function",
    kind = fn == print and "native" or "script",
    name = "oracle",
    identity = tostring(fn),
    raw = tostring(fn),
    params = 0,
    vararg = true,
    upvalues = 0,
  }
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

local pat = utf8.charpattern
assert(type(pat) == "string")
assert(#pat > 0)
assert(string.sub(pat, 1, 1) == "[")
assert(string.sub(pat, -1) == "*")

local mixed = "A" .. utf8.char(0x4e2d) .. utf8.char(0x1f600) .. "z"
assert(utf8.sub(mixed, -2, -1) == utf8.char(0x1f600) .. "z")
assert(utf8.sub(mixed, -3, -2) == utf8.char(0x4e2d) .. utf8.char(0x1f600))
assert(utf8.sub(mixed, -100, 2) == "A" .. utf8.char(0x4e2d))
assert(utf8.sub(mixed, 3, 99) == utf8.char(0x1f600) .. "z")
assert(utf8.sub(mixed, 5, 99) == "")

assert(utf8.charclass(46) == "P")
assert(utf8.charclass(36) == "P")
assert(utf8.charclass(0) == "O")

local before = testkit.snapshot()
local after = testkit.memory()
local delta = testkit.diff(before, after)
assert(type(delta.allocBytes) == "number")
assert(type(delta.allocKB) == "number")
assert(type(delta.sysBytes) == "number")
assert(type(delta.heapObjects) == "number")
assert(type(delta.numGC) == "number")
assert(type(delta.rootLog) == "number")
assert(type(delta.before) == "table")
assert(type(delta.after) == "table")

local ok, report = testkit.checkMemory(before, {
  collect = true,
  maxAllocBytesGrowth = -1,
  maxHeapObjectsGrowth = -1,
  maxRootLogGrowth = -1,
})
assert(not ok and report.ok == false)
assert(type(report.allocBytes) == "number")
assert(type(report.heapObjects) == "number")
assert(type(report.rootLog) == "number")

local numInfo = testkit.value(42)
local floatInfo = testkit.value(1.5)
local tableInfo = testkit.value({10, 20, 30})
local fnInfoValue = nil
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

local function pack(a, b, ...)
  return a, b, select("#", ...)
end

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
assert(not testkit.equal({}, {}))
local same = {}
assert(testkit.equal(same, same))

local good = testkit.protect(pack, "a", "b", "c", "d")
assert(good.ok and good.n == 3)
assert(good.values[1] == "a" and good.values[2] == "b" and good.values[3] == 2)

local function fail_string()
  error("planned failure")
end

local bad = testkit.protect(fail_string)
assert(not bad.ok)
assert(type(bad.error) == "string")
assert(string.find(bad.error, "planned failure", 1, true) ~= nil)

local info = testkit.functionInfo(pack)
assert(info.type == "function")
assert(info.kind == "script")
assert(type(info.name) == "string")
assert(type(info.identity) == "string")
assert(type(info.raw) == "string")
assert(type(info.params) == "number")
assert(type(info.vararg) == "boolean")
assert(type(info.upvalues) == "number")

local nativeInfo = testkit.functionInfo(print)
assert(nativeInfo.type == "function")
assert(nativeInfo.kind == "native")
assert(type(nativeInfo.name) == "string")
assert(type(nativeInfo.identity) == "string")
assert(type(nativeInfo.raw) == "string")

print("ok")
