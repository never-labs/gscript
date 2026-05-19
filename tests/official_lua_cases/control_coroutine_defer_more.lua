print("case:control_coroutine_defer_more")

local order = ""

local function mark(s)
  order = order .. s .. "|"
end

local function returnWithDefer(label)
  mark(label .. ":body")
  local ret = label .. ":value"
  mark(label .. ":second")
  mark(label .. ":first")
  return ret
end

local rv = returnWithDefer("ret")
assert(rv == "ret:value")
assert(order == "ret:body|ret:second|ret:first|")

local cfg = {count = 1, note = "outer"}
cfg.count = 2

local function mutateCaptured(delta)
  cfg.count = cfg.count + delta
  return cfg.count
end

assert(mutateCaptured(3) == 5)
local okAssign = pcall(function()
  error("cannot assign to readonly variable \"cfg\"")
end)
assert(not okAssign)

do
  local cfg = {count = 10}
  cfg.count = 11
  assert(cfg.count == 11)
end
assert(cfg.count == 5 and cfg.note == "outer")

local cachedYield = coroutine.yield
local cachedIsYieldable = coroutine.isyieldable
local co = coroutine.create(function(seed)
  assert(cachedIsYieldable())
  mark("co:start")
  local a, b = cachedYield("cached", seed + 1)
  assert(cachedIsYieldable())
  mark("co:after")
  return a + b, cfg.count
end)

local ok, tag, val = coroutine.resume(co, 41)
assert(ok and tag == "cached" and val == 42)
assert(coroutine.status(co) == "suspended")

local sum, capturedCount
ok, sum, capturedCount = coroutine.resume(co, 7, 8)
assert(ok and sum == 15 and capturedCount == 5)
assert(coroutine.status(co) == "dead")

local function protectedStep()
  mark("pcall:before")
  mark("pcall:defer2")
  mark("pcall:defer1")
  error("inner", 0)
end

local okInner, errInner = pcall(protectedStep)
assert(not okInner and errInner ~= nil)
mark("pcall:continued")

local function outerFailure()
  local function cleanup()
    mark("outer:second")
    mark("outer:first")
  end

  local val = returnWithDefer("nested")
  assert(val == "nested:value")
  mark("outer:before-error")
  cleanup()
  error("outer", 0)
end

local okOuter, errOuter = pcall(outerFailure)
assert(not okOuter and errOuter ~= nil)

local expected = "ret:body|ret:second|ret:first|" ..
  "co:start|co:after|" ..
  "pcall:before|pcall:defer2|pcall:defer1|pcall:continued|" ..
  "nested:body|nested:second|nested:first|outer:before-error|outer:second|outer:first|"
assert(order == expected)

print("ok")
