print("case:events_newindex_loop_more")

T, K, V = nil
grandparent := {}
grandparent.__newindex = func(t, k, v) { T = t; K = k; V = v }
parent := {}
parent.__newindex = parent
setmetatable(parent, grandparent)
child := {}
setmetatable(child, parent)
child.foo = 10
assert(T == parent && K == "foo" && V == 10)
T, K, V = nil

print("ok")
