print("case:api_metamethod_compare_len_concat_more")

do
  local mt = {
    __lt = function (a, b) return a[1] < b[1] end,
    __le = function (a, b) return a[1] <= b[1] end,
    __eq = function (a, b) return a[1] == b[1] end,
  }
  local function O (x)
    return setmetatable({x}, mt)
  end

  assert(O(1) < O(2))
  assert(not (O(3) <= O(2)))
  assert(O(3) == O(3))
  assert(O(4) > O(3))
  assert(O(4) >= O(4))
end

do
  local t = setmetatable({x = 20}, {__len = function (v) return v.x end})
  assert(#t == 20)
  t.x = "234"
  t[1] = 20
  assert(#t == "234")
end

do
  local a = setmetatable({x = "u"}, {
    __concat = function (l, r) return l.x .. "." .. r.x end,
  })
  assert(a .. a == "u.u")
  assert("" .. "xuxu" == "xuxu")
end

print("ok")
