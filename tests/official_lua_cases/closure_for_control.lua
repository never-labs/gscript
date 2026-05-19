print("case:closure_for_control")

local a = {}
for i = 1, 10 do
  local current = i
  a[i] = {
    set = function (x) current = x end,
    get = function () return current end,
  }
  if i == 3 then
    break
  end
end

assert(a[4] == nil)
a[1].set(10)
assert(a[2].get() == 2)
a[2].set("a")
assert(a[3].get() == 3)
assert(a[2].get() == "a")

print("ok")
