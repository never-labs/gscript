print("case:all_harness_flags_format")

local function defaults (opts, usertests)
  local _soft = rawget(opts, "_soft") or false
  local _port = rawget(opts, "_port") or false
  local _nomsg = rawget(opts, "_nomsg") or false

  if usertests then
    _soft = true
    _port = true
    _nomsg = true
  end

  local msgs = {}
  local function Message (m)
    if not _nomsg then
      msgs[#msgs + 1] = string.sub(m, 3, -3)
    end
  end

  local function F (m)
    local function round (m)
      m = m + 0.04999
      return string.format("%.1f", m)
    end
    if m < 1000 then return m
    else
      m = m / 1000
      if m < 1000 then return round(m) .. "K"
      else return round(m / 1000) .. "M"
      end
    end
  end

  Message("**one**")
  Message("**two**")
  return _soft, _port, _nomsg, #msgs, F(999), F(1000), F(1530), F(1000000), F(2500000)
end

local s, p, n, count, a, b, c, d, e = defaults({}, nil)
assert(not s and not p and not n)
assert(count == 2)
assert(a == 999 and b == "1.0K" and c == "1.6K")
assert(d == "1.0M" and e == "2.5M")

s, p, n, count = defaults({_soft = true}, true)
assert(s and p and n)
assert(count == 0)

print("ok")
