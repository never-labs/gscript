print("case:pm_malformed_pattern_errors_more")

func malform(p) {
  r := pcall(string.find, "a", p)
  assert(!r)
}

malform("(.")
malform(".)")
malform("[a")
malform("[]")
malform("[^]")
malform("[a%]")
malform("[a%")

print("ok")
