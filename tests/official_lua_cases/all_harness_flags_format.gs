print("case:all_harness_flags_format")

func defaults(opts, usertests) {
  soft := rawget(opts, "_soft")
  if soft == nil { soft = false }
  port := rawget(opts, "_port")
  if port == nil { port = false }
  nomsg := rawget(opts, "_nomsg")
  if nomsg == nil { nomsg = false }

  if usertests {
    soft = true
    port = true
    nomsg = true
  }

  msgs := {}
  Message := func(m) {
    if !nomsg {
      msgs[#msgs + 1] = string.sub(m, 3, -3)
    }
  }

  F := func(m) {
    round := func(m) {
      m = m + 0.04999
      return string.format("%.1f", m)
    }
    if m < 1000 {
      return m
    }
    m = m / 1000
    if m < 1000 {
      return round(m) .. "K"
    }
    return round(m / 1000) .. "M"
  }

  Message("**one**")
  Message("**two**")
  return soft, port, nomsg, #msgs, F(999), F(1000), F(1530), F(1000000), F(2500000)
}

s, p, n, count, a, b, c, d, e := defaults({}, nil)
assert(!s && !p && !n)
assert(count == 2)
assert(a == 999 && b == "1.0K" && c == "1.6K")
assert(d == "1.0M" && e == "2.5M")

s, p, n, count = defaults({_soft: true}, true)
assert(s && p && n)
assert(count == 0)

print("ok")
