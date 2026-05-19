print("case:sort_binary_string_order_more")

a := {"alo", "\0first :-)", "then this one", "45", "and a new"}
table.sort(a)

for i := 2; i <= #a; i += 1 {
  assert(a[i - 1] <= a[i])
}
assert(a[1] == "\0first :-)" && a[2] == "45" && a[#a] == "then this one")

print("ok")
