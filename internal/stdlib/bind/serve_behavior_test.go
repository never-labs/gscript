package bind

import "testing"

func TestServeAppRoutesBackgroundRoundTrip(t *testing.T) {
	interp := New()
	hostOpts := HostOptions{
		Call:           interp.CallFunction,
		NetworkAllowed: interp.NetworkAccessEnabled,
		MaxHostResult:  interp.MaxHostResultBytes,
	}
	httpModule := TableValue(BuildHTTP(hostOpts))
	serveModule := TableValue(BuildServe(hostOpts))
	interp.SetGlobal("http", httpModule)
	interp.SetModule("http", httpModule)
	interp.SetGlobal("serve", serveModule)
	interp.SetModule("serve", serveModule)

	src := `
		server := serve.app({
			listen: "127.0.0.1:0",
			routes: {
				{method: "GET", path: "/", handler: func(req) {
					return "hello"
				}},
				{method: "GET", path: "/users/:id", handler: func(req) {
					return {id: req.params.id}
				}},
				{method: "POST", path: "/echo", handler: func(req) {
					body, err := req.json()
					if err != nil {
						return {error: err}
					}
					return body
				}},
			},
		})
		root := http.get(server.url .. "/")
		user := http.get(server.url .. "/users/ada")
		echo := net.post(server.url .. "/echo", "{\"ok\":true}", {headers: {["Content-Type"]: "application/json"}})
		closed, closeErr := server.close()
		waited, waitErr := server.wait()
	`
	netModule := TableValue(BuildNet(hostOpts))
	interp.SetGlobal("net", netModule)
	interp.SetModule("net", netModule)
	if _, err := interp.ExecString(src); err != nil {
		t.Fatalf("ExecString: %v", err)
	}
	if got := interp.GetGlobal("root").Table().RawGetString("body").Str(); got != "hello" {
		t.Fatalf("root body = %q, want hello", got)
	}
	if got := interp.GetGlobal("user").Table().RawGetString("body").Str(); got != `{"id":"ada"}` {
		t.Fatalf("user body = %q, want JSON id", got)
	}
	if got := interp.GetGlobal("echo").Table().RawGetString("body").Str(); got != `{"ok":true}` {
		t.Fatalf("echo body = %q, want echoed JSON", got)
	}
	for _, name := range []string{"closed", "waited"} {
		if got := interp.GetGlobal(name); !got.IsBool() || !got.Bool() {
			t.Fatalf("%s = %v, want true", name, got)
		}
	}
	for _, name := range []string{"closeErr", "waitErr"} {
		if got := interp.GetGlobal(name); !got.IsNil() {
			t.Fatalf("%s = %v, want nil", name, got)
		}
	}
}

func TestHTTPRouterColonParams(t *testing.T) {
	interp := New()
	hostOpts := HostOptions{
		Call:           interp.CallFunction,
		NetworkAllowed: interp.NetworkAccessEnabled,
		MaxHostResult:  interp.MaxHostResultBytes,
	}
	httpModule := TableValue(BuildHTTP(hostOpts))
	interp.SetGlobal("http", httpModule)
	interp.SetModule("http", httpModule)
	src := `
		router := http.newRouter()
		router.get("/users/:id", func(req, res) {
			res.json({id: req.params.id})
		})
		server := router.listen("127.0.0.1:0", {background: true})
		resp := http.get(server.url .. "/users/bob")
		closed, closeErr := server.close()
		waited, waitErr := server.wait()
	`
	if _, err := interp.ExecString(src); err != nil {
		t.Fatalf("ExecString: %v", err)
	}
	if got := interp.GetGlobal("resp").Table().RawGetString("body").Str(); got != `{"id":"bob"}` {
		t.Fatalf("resp body = %q, want JSON id", got)
	}
}
