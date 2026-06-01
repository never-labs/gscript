print("case:errors_pcall_xpcall_values")

ok, a, b, c := pcall(assert, true, "a", "b")
assert(ok && a == true && b == "a" && c == "b")

err := {tag: "error-object"}
ok, a = pcall(error, err)
assert(!ok && a == err)

ok, a = xpcall(error, tostring, "xpcall-error")
assert(!ok && a == "xpcall-error")

handled := nil
func handler(e) {
  handled = e
  return e.tag
}

ok, a = xpcall(error, handler, err)
assert(!ok && handled == err && a == "error-object")

print("ok")
