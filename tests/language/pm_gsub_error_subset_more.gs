print("case:pm_gsub_error_subset_more")

func checkerror(msg, f, ...) {
  s, err := pcall(f, ...)
  assert(!s && string.find(err, msg))
}

checkerror("invalid capture index %%2", string.gsub, "alo", ".", "%2")
checkerror("invalid use of '%%'", string.gsub, "alo", ".", "%x")

print("ok")
