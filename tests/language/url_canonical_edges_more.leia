print("case:url_canonical_edges_more")

u := url.parse("https://user@[2001:db8::1]:8443/a%2Fb/c%20d?q=&q=second&space=a+b&encoded=%2Fpath%3Fx%3D1#frag%20ment")
assert(u.scheme == "https")
assert(u.host == "2001:db8::1")
assert(u.port == "8443")
assert(u.path == "/a/b/c d")
assert(u.fragment == "frag ment")
assert(u.user == "user")
assert(u.password == nil)
assert(u.query.q == "")
assert(u.query.space == "a b")
assert(u.query.encoded == "/path?x=1")
assert(u.raw == "https://user@[2001:db8::1]:8443/a%2Fb/c%20d?q=&q=second&space=a+b&encoded=%2Fpath%3Fx%3D1#frag%20ment")

built := url.build({
	scheme: "https",
	host: "2001:db8::1",
	port: "8443",
	path: "/a/b c/%2F",
	user: "user",
	query: {empty: "", path: "/a b"},
	fragment: "frag ment",
})
assert(built == "https://user@2001:db8::1:8443/a/b%20c/%252F?empty=&path=%2Fa+b#frag%20ment")

builtBracketed := url.build({
	scheme: "https",
	host: "[2001:db8::1]",
	port: "8443",
	path: "/a/b c/%2F",
	user: "user",
	query: {empty: "", path: "/a b"},
	fragment: "frag ment",
})
assert(builtBracketed == "https://user@[2001:db8::1]:8443/a/b%20c/%252F?empty=&path=%2Fa+b#frag%20ment")

query := url.queryEncode({empty: "", path: "/a b", space: "a b"})
assert(query == "empty=&path=%2Fa+b&space=a+b")
qt, err := url.queryDecode("dup=first&dup=second&empty=&bare&encoded=%2Fpath%3Fx%3D1")
assert(err == nil)
assert(qt.dup == "first")
assert(qt.empty == "")
assert(qt.bare == "")
assert(qt.encoded == "/path?x=1")

badQuery, badErr := url.queryDecode("%zz")
assert(badQuery == nil)
assert(type(badErr) == "string")

assert(url.join("https://example.com/base/path?q=old", "https://other.example/x/y?z=1") == "https://other.example/x/y?z=1")
assert(url.getHost("https://user@[2001:db8::1]:8443/a%2Fb/c%20d?q=1") == "2001:db8::1")
assert(url.getPath("https://user@[2001:db8::1]:8443/a%2Fb/c%20d?q=1") == "/a/b/c d")
assert(url.isValid("https://[2001:db8::1]:8443/a%2Fb") == true)
assert(url.isValid("//example.com/path") == false)
assert(url.isValid("https://[::1") == false)

print("ok")
