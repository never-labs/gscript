print("case:nextvar_length_more")

assert(#{} == 0)
assert(#{nil} == 0)
assert(#{nil, nil} == 0)
assert(#{nil, nil, nil} == 0)
assert(#{nil, nil, nil, nil} == 0)
assert(#{1, 2, 3, nil, nil} == 3)

t := {}
t[-1] = 2
assert(#t == 0)

for i := 0; i <= 40; i++ {
  a := {}
  for j := 1; j <= i; j++ {
    a[j] = j
  }
  assert(#a == i)
}

print("ok")
