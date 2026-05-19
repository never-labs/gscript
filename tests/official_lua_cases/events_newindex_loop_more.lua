print("case:events_newindex_loop_more")

T, K, V = nil
local grandparent = {}
grandparent.__newindex = function(t, k, v) T = t; K = k; V = v end
local parent = {}
parent.__newindex = parent
setmetatable(parent, grandparent)
local child = setmetatable({}, parent)
child.foo = 10
assert(T == parent and K == "foo" and V == 10)
T, K, V = nil

print("ok")
