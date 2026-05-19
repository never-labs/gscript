local order = ""

local function record(s)
  order = order .. s
end

local function work()
  local cfg = {count = 1}
  cfg.count = 2
  local ok = pcall(function()
    error("cannot assign to readonly variable \"cfg\"")
  end)
  if ok then
    record("bad")
  end
  record("body")
  record("two")
  record("one")
  error("boom", 0)
end

local ok, err = pcall(work)
print(ok, err, order)
