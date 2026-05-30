print("case:goto_simple_paths_more")

local x
do
  local y = 12
  goto l1
  ::l2:: x = x + 1; goto l3
  ::l1:: x = y; goto l2
end
::l3:: assert(x == 13)

local function foo()
  local a = {}
  goto l3
  ::l1:: a[#a + 1] = 1; goto l2
  ::l2:: a[#a + 1] = 2; goto l5
  ::l3:: a[#a + 1] = 3; goto l1
  ::l4:: a[#a + 1] = 4; goto l6
  ::l5:: a[#a + 1] = 5; goto l4
  ::l6:: return table.concat(a, ",")
end

assert(foo() == "3,1,2,5,4")

print("ok")
