print("case:coroutine_defer_xpcall_edges_more")

local order = ""

local function mark(s)
  order = order .. s .. "|"
end

local co = coroutine.create(function(seed)
  mark("co-return:start")
  local yielded = coroutine.yield("yielded", seed + 1)
  mark("co-return:after-yield")
  mark("co-return:defer2")
  mark("co-return:defer1")
  return yielded * 2
end)

local ok, tag, value = coroutine.resume(co, 10)
assert(ok and tag == "yielded" and value == 11)
assert(coroutine.status(co) == "suspended")
assert(order == "co-return:start|")

ok, value = coroutine.resume(co, 21)
assert(ok and value == 42)
assert(coroutine.status(co) == "dead")
assert(order == "co-return:start|co-return:after-yield|co-return:defer2|co-return:defer1|")

local errco = coroutine.create(function()
  mark("co-error:start")
  coroutine.yield("pause")
  mark("co-error:after-yield")
  mark("co-error:defer2")
  mark("co-error:defer1")
  error("co-error-boom", 0)
end)

ok, tag = coroutine.resume(errco)
assert(ok and tag == "pause")
assert(coroutine.status(errco) == "suspended")
assert(order == "co-return:start|co-return:after-yield|co-return:defer2|co-return:defer1|co-error:start|")

local err
ok, err = coroutine.resume(errco)
assert(not ok and err ~= nil)
assert(coroutine.status(errco) == "dead")
assert(order == "co-return:start|co-return:after-yield|co-return:defer2|co-return:defer1|co-error:start|co-error:after-yield|co-error:defer2|co-error:defer1|")

local phase = "init"
local drains = 0

local function protectedBoom(label)
  phase = label .. ":body"
  mark(label .. ":before-error")
  drains = drains + 1
  phase = label .. ":cleanup2"
  mark(label .. ":defer2")
  drains = drains + 1
  phase = label .. ":cleanup1"
  mark(label .. ":defer1")
  error(label .. ":boom", 0)
end

local res, msg = xpcall(function()
  protectedBoom("xp-ok")
end, function(e)
  mark("xp-ok:handler")
  assert(drains == 2)
  assert(phase == "xp-ok:cleanup1")
  return "handled:" .. phase
end)

assert(not res and msg == "handled:xp-ok:cleanup1")

drains = 0
phase = "reset"
local handlerCalls = 0
res, msg = xpcall(function()
  protectedBoom("xp-handler-error")
end, function(e)
  handlerCalls = handlerCalls + 1
  if handlerCalls > 1 then
    return "handler-recursed"
  end
  mark("xp-handler-error:handler")
  assert(drains == 2)
  assert(phase == "xp-handler-error:cleanup1")
  error("handler-boom", 0)
end)

assert(not res and drains == 2)

local expected = "co-return:start|co-return:after-yield|co-return:defer2|co-return:defer1|" ..
  "co-error:start|co-error:after-yield|co-error:defer2|co-error:defer1|" ..
  "xp-ok:before-error|xp-ok:defer2|xp-ok:defer1|xp-ok:handler|" ..
  "xp-handler-error:before-error|xp-handler-error:defer2|xp-handler-error:defer1|xp-handler-error:handler|"
assert(order == expected)

print("ok")
