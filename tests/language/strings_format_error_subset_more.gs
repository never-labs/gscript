print("case:strings_format_error_subset_more")

func checkerror(msg, f, ...) {
  s, err := pcall(f, ...)
  assert(!s && string.find(err, msg))
}

checkerror("invalid", string.format, "%t", 10)
checkerror("no value", string.format, "%d %d", 10)
checkerror("invalid", string.format, "%F", 10)

print("ok")
