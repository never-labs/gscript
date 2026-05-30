print("case:defer_loader_require_edges_more")

order := ""

func record(label, value) {
	order = order .. label .. "=" .. tostring(value) .. "|"
}

func deferArgsEvaluateAtRegistration() {
	label := "first"
	value := 10
	defer record(label, value)

	label = "second"
	value = 20
	defer record(label, value)

	label = "third"
	value = 30
	return "done"
}

assert(deferArgsEvaluateAtRegistration() == "done")
assert(order == "second=20|first=10|")

httpMod := require("http")
netMod := require("net")
vecMod := require("vec")
colorMod := require("color")

assert(httpMod == http)
assert(netMod == net)
assert(vecMod == vec)
assert(colorMod == color)

assert(package.loaded.http == http)
assert(package.loaded.net == net)
assert(package.loaded.vec == vec)
assert(package.loaded.color == color)

assert(require("http") == httpMod)
assert(require("net") == netMod)
assert(require("vec") == vecMod)
assert(require("color") == colorMod)

bad, err := load("return !", "defer_loader_require_edges_more_source.gs")
assert(bad == nil)
assert(type(err) == "string")
assert(string.find(err, "defer_loader_require_edges_more_source.gs", 1, true) != nil)

print("ok")
