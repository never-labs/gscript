print("case:nextvar_numeric_for_float_counts")

a := 0
for i := 1; i <= 1; i = i + 1 { a = a + 1 }
assert(a == 1)

a = 0
for i := 10000; i >= 1e4; i = i - 1 { a = a + 1 }
assert(a == 1)

a = 0
for i := 1; i <= 0.99999; i = i + 1 { a = a + 1 }
assert(a == 0)

a = 0
for i := 9999; i >= 1e4; i = i - 1 { a = a + 1 }
assert(a == 0)

a = 0
for i := 1; i >= 0.99999; i = i - 1 { a = a + 1 }
assert(a == 1)

a = 0
for i := 0; i <= 0.999999999; i = i + 0.1 { a = a + 1 }
assert(a == 10)

a = 0
for i := 1.0; i <= 1; i = i + 1 { a = a + 1 }
assert(a == 1)

a = 0
for i := -1.5; i <= -1.5; i = i + 1 { a = a + 1 }
assert(a == 1)

a = 0
for i := 1e6; i >= 1e6; i = i - 1 { a = a + 1 }
assert(a == 1)

a = 0
for i := 1.0; i <= 0.99999; i = i + 1 { a = a + 1 }
assert(a == 0)

a = 0
for i := 99999; i >= 1e5; i = i - 1.0 { a = a + 1 }
assert(a == 0)

a = 0
for i := 1.0; i >= 0.99999; i = i - 1 { a = a + 1 }
assert(a == 1)

print("ok")
