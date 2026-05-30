print("case:sort_month_names_more")

local function check(a, f)
  f = f or function (x, y) return x < y end
  for n = #a, 2, -1 do
    assert(not f(a[n], a[n - 1]))
  end
end

a = {"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep",
     "Oct", "Nov", "Dec"}
table.sort(a)
check(a)
assert(table.concat(a, ",") == "Apr,Aug,Dec,Feb,Jan,Jul,Jun,Mar,May,Nov,Oct,Sep")

print("ok")
