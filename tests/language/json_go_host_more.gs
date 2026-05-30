print("case:json_go_host_more")

doc := json.decode("{\"name\":\"gscript\",\"items\":[1,2,3],\"flag\":true,\"nested\":{\"x\":4}}")
assert(doc.name == "gscript")
assert(doc.items[1] == 1)
assert(doc.items[3] == 3)
assert(doc.flag == true)
assert(doc.nested.x == 4)

encoded := json.encode({name: "gscript", items: {1, 2, 3}})
round := json.decode(encoded)
assert(round.name == "gscript")
assert(round.items[2] == 2)

pretty := json.pretty({outer: {value: 7}}, "  ")
assert(string.find(pretty, "\n", 1, true) != nil)
assert(string.find(pretty, "\"outer\"", 1, true) != nil)

bad, err := json.decode("{\"ok\":true} trailing")
assert(bad == nil)
assert(type(err) == "string")

bad, err = json.decode("{")
assert(bad == nil)
assert(type(err) == "string")

print("ok")
