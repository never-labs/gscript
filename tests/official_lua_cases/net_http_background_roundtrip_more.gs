print("case:net_http_background_roundtrip_more")

server := http.listen("127.0.0.1:0", func(req, res) {
	if req.path == "/json" {
		body := req.json()
		res.status(201).json({method: req.method, name: body.name})
	} elseif req.path == "/missing" {
		res.status(404).write("missing")
	} else {
		res.write(req.method .. ":" .. req.path .. ":" .. req.body)
	}
}, {background: true})

resp, err := net.get(server.url .. "/hello")
assert(err == nil)
assert(resp.status == 200)
assert(resp.statusText == "200 OK")
assert(resp.body == "GET:/hello:")
assert(resp.ok == true)
assert(type(resp.headers) == "table")
assert(type(resp.json) == "function")

headers := {}
headers["Content-Type"] = "application/json"
resp, err = net.post(server.url .. "/json", "{\"name\":\"gscript\"}", {headers: headers})
assert(err == nil)
assert(resp.status == 201)
data, jsonErr := resp.json()
assert(jsonErr == nil)
assert(data.method == "POST")
assert(data.name == "gscript")

resp, err = net.request({url: server.url .. "/missing"})
assert(err == nil)
assert(resp.status == 404)
assert(resp.ok == false)
assert(resp.body == "missing")

bad, parseErr := resp.json()
assert(bad == nil)
assert(type(parseErr) == "string")

badReq, badErr := net.get("http://[::1")
assert(badReq == nil)
assert(type(badErr) == "string")

ok, perr := pcall(net.request, {})
assert(ok == false)
assert(type(perr) == "string")

closed, closeErr := server.close()
assert(closed == true)
assert(closeErr == nil)
waited, waitErr := server.wait()
assert(waited == true)
assert(waitErr == nil)

print("ok")
