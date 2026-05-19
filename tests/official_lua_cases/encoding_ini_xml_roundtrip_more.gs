print("case:encoding_ini_xml_roundtrip_more")

cfg := {
  name: "gscript",
  enabled: true,
  server: {
    host: "127.0.0.1",
    port: 8080,
  },
  paths: {
    root: "/tmp/app",
    mode: "roundtrip",
  },
}

ini := encoding.iniEncode(cfg)
assert(string.find(ini, "name=gscript", 1, true) != nil)
assert(string.find(ini, "enabled=true", 1, true) != nil)
assert(string.find(ini, "[server]", 1, true) != nil)
assert(string.find(ini, "host=127.0.0.1", 1, true) != nil)
assert(string.find(ini, "port=8080", 1, true) != nil)
assert(string.find(ini, "[paths]", 1, true) != nil)
assert(string.find(ini, "root=/tmp/app", 1, true) != nil)
assert(string.find(ini, "mode=roundtrip", 1, true) != nil)

decoded := encoding.iniDecode(ini)
assert(decoded.name == "gscript")
assert(decoded.enabled == "true")
assert(decoded.server.host == "127.0.0.1")
assert(decoded.server.port == "8080")
assert(decoded.paths.root == "/tmp/app")
assert(decoded.paths.mode == "roundtrip")

manual := encoding.iniDecode("# comment\n title = hello world \n[db]\n user = root \n password = pa=ss \n")
assert(manual.title == "hello world")
assert(manual.db.user == "root")
assert(manual.db.password == "pa=ss")

xml := "<node attr=\"a&b\">Tom & 'Jerry'</node>"
escaped := encoding.xmlEscape(xml)
assert(escaped == "&lt;node attr=&#34;a&amp;b&#34;&gt;Tom &amp; &#39;Jerry&#39;&lt;/node&gt;")
unescaped, err := encoding.xmlUnescape(escaped)
assert(err == nil)
assert(unescaped == xml)
assert(encoding.xmlUnescape("&lt;x&gt;&quot;q&quot; &apos;s&apos;&lt;/x&gt;") == "<x>\"q\" 's'</x>")
assert(encoding.xmlUnescape("&amp;lt;") == "&lt;")

bad, err := encoding.base32Decode("NBSWY3D!")
assert(bad == nil)
assert(type(err) == "string")
assert(string.find(err, "illegal base32", 1, true) != nil)

bad, err = encoding.base32HexDecode("CTN!")
assert(bad == nil)
assert(type(err) == "string")
assert(string.find(err, "illegal base32", 1, true) != nil)

print("ok")
