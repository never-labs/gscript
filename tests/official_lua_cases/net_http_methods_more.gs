print("case:net_http_methods_more")

server := http.listen("127.0.0.1:0", func(req, res) {
	if req.path == "/put" {
		res.header("X-Seen-Method", req.method)
		res.write(req.method .. ":" .. req.body .. ":" .. req.headers["X-Client"])
	} elseif req.path == "/patch" {
		res.write(req.method .. ":" .. req.body .. ":" .. req.headers["Content-Type"])
	} elseif req.path == "/delete" {
		res.status(202).write(req.method .. ":" .. req.body .. ":" .. req.headers["X-Delete"])
	} elseif req.path == "/request" {
		res.status(206).write(req.method .. ":" .. req.body .. ":" .. req.headers["X-Request"])
	} elseif req.path == "/redirect" {
		res.redirect("/target", 302)
	} elseif req.path == "/target" {
		res.write("followed")
	} else {
		res.status(404).write("missing")
	}
}, {background: true})

headers := {}
headers["X-Client"] = "put-one"
resp, err := net.put(server.url .. "/put", "put-body", {headers: headers, timeout: 2})
assert(err == nil)
assert(resp.status == 200)
assert(resp.headers["X-Seen-Method"] == "PUT")
assert(resp.body == "PUT:put-body:put-one")

headers = {}
headers["Content-Type"] = "text/plain"
resp, err = net.patch(server.url .. "/patch", "patch-body", {headers: headers})
assert(err == nil)
assert(resp.status == 200)
assert(resp.body == "PATCH:patch-body:text/plain")

headers = {}
headers["X-Delete"] = "delete-one"
resp, err = net.delete(server.url .. "/delete", {headers: headers})
assert(err == nil)
assert(resp.status == 202)
assert(resp.ok == true)
assert(resp.body == "DELETE::delete-one")

headers = {}
headers["X-Request"] = "custom-one"
resp, err = net.request({
	method: "put",
	url: server.url .. "/request",
	headers: headers,
	body: "request-body",
	timeout: 2,
})
assert(err == nil)
assert(resp.status == 206)
assert(resp.body == "PUT:request-body:custom-one")

resp, err = net.request({
	method: "GET",
	url: server.url .. "/redirect",
	followRedirects: false,
})
assert(err == nil)
assert(resp.status == 302)
assert(resp.ok == true)
assert(resp.headers["Location"] == "/target")

resp, err = net.request({
	method: "GET",
	url: server.url .. "/redirect",
})
assert(err == nil)
assert(resp.status == 200)
assert(resp.body == "followed")

closed, closeErr := server.close()
assert(closed == true)
assert(closeErr == nil)
waited, waitErr := server.wait()
assert(waited == true)
assert(waitErr == nil)

print("ok")
