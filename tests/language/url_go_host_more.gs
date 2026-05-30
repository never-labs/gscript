print("case:url_go_host_more")

u := url.parse("https://user:pass@example.com:8443/a/b?q=hello+world&x=1#frag")
assert(u.scheme == "https")
assert(u.host == "example.com")
assert(u.port == "8443")
assert(u.path == "/a/b")
assert(u.fragment == "frag")
assert(u.user == "user")
assert(u.password == "pass")
assert(u.query.q == "hello world")
assert(u.query.x == "1")

built := url.build({
	scheme: "https",
	host: "example.com",
	port: "8443",
	path: "/a/b",
	query: {q: "hello world", x: "1"},
	fragment: "frag",
})
assert(built == "https://example.com:8443/a/b?q=hello+world&x=1#frag")

assert(url.encode("a b+c") == "a+b%2Bc")
decoded, err := url.decode("a+b%2Bc")
assert(decoded == "a b+c")
assert(err == nil)
decoded, err = url.decode("%zz")
assert(decoded == nil)
assert(type(err) == "string")

query := url.queryEncode({q: "hello world", x: "1"})
assert(query == "q=hello+world&x=1")
qt, err := url.queryDecode(query)
assert(err == nil)
assert(qt.q == "hello world")
assert(qt.x == "1")

assert(url.join("https://example.com/a/b/", "../c?q=1") == "https://example.com/a/c?q=1")
assert(url.isValid("https://example.com/a") == true)
assert(url.isValid("/relative") == false)
assert(url.getHost("https://example.com/a") == "example.com")
assert(url.getPath("https://example.com/a/b") == "/a/b")

print("ok")
