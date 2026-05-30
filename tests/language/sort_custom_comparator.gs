print("case:sort_custom_comparator")

func less(x, y) {
  return x < y
}

func greater(x, y) {
  return y < x
}

func check(a, f) {
  for n := #a; n >= 2; n = n - 1 {
    assert(!f(a[n], a[n - 1]))
  }
}

a := {5, 1, 4, 2, 3}
count := 0
func desc(x, y) {
  count = count + 1
  return y < x
}
table.sort(a, desc)
assert(count > 0)
check(a, greater)
assert(table.concat(a, ",") == "5,4,3,2,1")

table.sort({})

a = {false, false, false, false}
func equal_cmp(x, y) {
  return nil
}
table.sort(a, equal_cmp)
for i := 1; i <= #a; i = i + 1 {
  assert(a[i] == false)
}

words := {"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
table.sort(words)
check(words, less)

print("ok")
