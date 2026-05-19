print("case:big_table_sparse_pressure_more2")

t := {}
for i := 1; i <= 6000; i++ {
  t[i] = i % 97
}

for i := 1; i <= 6000; i = i + 3 {
  t[i] = nil
}

sum := 0
live := 0
for i := 1; i <= 6000; i++ {
  if t[i] != nil {
    sum = sum + t[i]
    live = live + 1
  }
}

assert(live == 4000)
assert(sum == 191728)
assert(t[2] == 2 && t[3] == 3 && t[4] == nil)
assert(t[5999] == 82 && t[6000] == 83)

print("ok")
