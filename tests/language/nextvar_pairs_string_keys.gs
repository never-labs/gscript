print("case:nextvar_pairs_string_keys")

t := {a: 10, b: 20, c: 30}
sum := 0
seen := {}

for k, v := range pairs(t) {
  seen[k] = true
  sum = sum + v
}

assert(sum == 60)
assert(seen.a && seen.b && seen.c)

count := 0
k := nil
for {
  k = next(t, k)
  if k != nil {
    count = count + 1
  } else {
    break
  }
}

assert(count == 3)

print("ok")
