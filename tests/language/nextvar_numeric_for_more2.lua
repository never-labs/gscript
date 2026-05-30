print("case:nextvar_numeric_for_more2")

local a
a = 0; for i = 1, 1, 1 do a = a + 1 end; assert(a == 1)
a = 0; for i = 10000, 1e4, -1 do a = a + 1 end; assert(a == 1)
a = 0; for i = 1, 0.99999, 1 do a = a + 1 end; assert(a == 0)
a = 0; for i = 9999, 1e4, -1 do a = a + 1 end; assert(a == 0)
a = 0; for i = 1, 0.99999, -1 do a = a + 1 end; assert(a == 1)

a = 0; for i = 0, 0.999999999, 0.1 do a = a + 1 end; assert(a == 10)
a = 0; for i = 1.0, 1, 1 do a = a + 1 end; assert(a == 1)
a = 0; for i = -1.5, -1.5, 1 do a = a + 1 end; assert(a == 1)
a = 0; for i = 1e6, 1e6, -1 do a = a + 1 end; assert(a == 1)

print("ok")
