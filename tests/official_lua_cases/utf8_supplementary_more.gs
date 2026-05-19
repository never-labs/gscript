print("case:utf8_supplementary_more")

s := "𣲷𠜎𠱓𡁻𠵼ab𠺢"
t := {146615, 132878, 134227, 135291, 134524, 97, 98, 134818}
assert(utf8.len(s) == #t)
for i := 1; i <= #t; i++ {
  p := utf8.offset(s, i)
  assert(utf8.codepoint(s, p) == t[i])
}
i := 0
for p, c := range utf8.codes(s) {
  i = i + 1
  assert(p == utf8.offset(s, i))
  assert(c == t[i])
}
assert(i == #t)

print("ok")
