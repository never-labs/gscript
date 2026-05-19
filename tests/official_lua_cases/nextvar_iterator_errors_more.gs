print("case:nextvar_iterator_errors_more")

func checkerror(msg, f, ...) {
  s, err := pcall(f, ...)
  assert(!s && string.find(err, msg))
}

assert(next({}) == next({}))
checkerror("invalid key", next, {10, 20}, 3)
checkerror("bad argument", pairs)
checkerror("bad argument", ipairs)
assert(next({}) == nil)
assert(next({}, nil) == nil)

print("ok")
