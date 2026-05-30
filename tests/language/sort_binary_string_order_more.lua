print("case:sort_binary_string_order_more")

local a = {"alo", "\0first :-)", "then this one", "45", "and a new"}
table.sort(a)

for i = 2, #a do
  assert(a[i - 1] <= a[i])
end
assert(a[1] == "\0first :-)" and a[2] == "45" and a[#a] == "then this one")

print("ok")
