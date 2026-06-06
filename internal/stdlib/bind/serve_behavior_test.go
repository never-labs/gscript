package bind

import (
	"strings"
	"testing"
)

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

func TestServeDialectTaggedBlockRoundTrip(t *testing.T) {
	interp := New()
	hostOpts := HostOptions{
		Call:           interp.CallFunction,
		NetworkAllowed: interp.NetworkAccessEnabled,
		MaxHostResult:  interp.MaxHostResultBytes,
	}
	httpModule := TableValue(BuildHTTP(hostOpts))
	netModule := TableValue(BuildNet(hostOpts))
	dialectModule := TableValue(BuildDialect(hostOpts, interp.MaxHostResultBytes))
	interp.SetGlobal("http", httpModule)
	interp.SetModule("http", httpModule)
	interp.SetGlobal("net", netModule)
	interp.SetModule("net", netModule)
	interp.SetGlobal("dialect", dialectModule)
	interp.SetModule("dialect", dialectModule)

	src := `
		app := serve {
			listen: "127.0.0.1:0"
			routes: {
				{method: "GET", path: "/pages/:name", handler: func(req) {
					return "<h1>" .. req.params.name .. ":" .. req.query.mode .. "</h1>"
				}},
				{method: "POST", path: "/api/items", handler: func(req) {
					body, err := req.json()
					if err != nil { return {error: err} }
					return {created: body.name, method: req.method}
				}},
			}
		}
		page := http.get(app.url .. "/pages/home?mode=full")
		created, createdErr := net.post(app.url .. "/api/items", "{\"name\":\"book\"}", {headers: {["Content-Type"]: "application/json"}})
		wrong, wrongErr := net.delete(app.url .. "/api/items")
		closed, closeErr := app.close()
		waited, waitErr := app.wait()
	`
	if _, err := interp.ExecString(src); err != nil {
		t.Fatalf("ExecString: %v", err)
	}
	if got := interp.GetGlobal("page").Table().RawGetString("body").Str(); got != "<h1>home:full</h1>" {
		t.Fatalf("page body = %q, want tagged serve HTML", got)
	}
	if got := interp.GetGlobal("created").Table().RawGetString("body").Str(); got != `{"created":"book","method":"POST"}` {
		t.Fatalf("created body = %q, want JSON create", got)
	}
	if got := interp.GetGlobal("wrong").Table().RawGetString("status").Int(); got != 405 {
		t.Fatalf("wrong status = %d, want 405", got)
	}
	for _, name := range []string{"createdErr", "wrongErr", "closeErr", "waitErr"} {
		if got := interp.GetGlobal(name); !got.IsNil() {
			t.Fatalf("%s = %v, want nil", name, got)
		}
	}
	for _, name := range []string{"closed", "waited"} {
		if got := interp.GetGlobal(name); !got.IsBool() || !got.Bool() {
			t.Fatalf("%s = %v, want true", name, got)
		}
	}
}

func TestServeDialectSQLiteFormAndStaticRoundTrip(t *testing.T) {
	interp := New()
	hostOpts := HostOptions{
		Call:           interp.CallFunction,
		NetworkAllowed: interp.NetworkAccessEnabled,
		MaxHostResult:  interp.MaxHostResultBytes,
	}
	for name, module := range map[string]Value{
		"db":      TableValue(BuildDB(hostOpts)),
		"dialect": TableValue(BuildDialect(hostOpts, interp.MaxHostResultBytes)),
		"http":    TableValue(BuildHTTP(hostOpts)),
		"net":     TableValue(BuildNet(hostOpts)),
	} {
		interp.SetGlobal(name, module)
		interp.SetModule(name, module)
	}

	src := `
		conn := db.memory()
		conn.exec(sql` + "`" + `create table posts (id integer primary key autoincrement, title text not null, body text not null)` + "`" + `)
		conn.exec(sql {query: "insert into posts (title, body) values (:title, :body)", params: {title: "seed", body: "hello"}})
		app := serve {
			listen: "127.0.0.1:0"
			routes: {
				{method: "GET", path: "/static/:name", handler: func(req) {
					return "asset:" .. req.params.name
				}},
				{method: "GET", path: "/api/posts/:id", handler: func(req) {
					rows := conn.query(sql {query: "select id, title, body from posts where id = :id", params: {id: req.params.id}})
					row := rows[1]
					return {id: row.id, title: row.title, body: row.body}
				}},
				{method: "POST", path: "/posts", handler: func(req) {
					form_body := dialect.eval("form", req.body, {mode: "parse"})
					inserted := conn.exec(sql {
						query: "insert into posts (title, body) values (:title, :body)",
						params: {title: form_body.title, body: form_body.body},
					})
					rows := conn.query(sql {query: "select id, title, body from posts where id = :id", params: {id: inserted.last_insert_id}})
					row := rows[1]
					return {id: row.id, title: row.title, body: row.body}
				}},
			}
		}
		static_resp := http.get(app.url .. "/static/app.css")
		api_resp := http.get(app.url .. "/api/posts/1")
		form_resp, form_err := net.post(app.url .. "/posts", form {title: "form title", body: "form body"}, {headers: {["Content-Type"]: "application/x-www-form-urlencoded"}})
		count_row := conn.one(sql` + "`" + `select count(*) as n from posts` + "`" + `)
		closed, close_err := app.close()
		waited, wait_err := app.wait()
	`
	if _, err := interp.ExecString(src); err != nil {
		t.Fatalf("ExecString: %v", err)
	}
	if got := interp.GetGlobal("static_resp").Table().RawGetString("body").Str(); got != "asset:app.css" {
		t.Fatalf("static body = %q, want asset", got)
	}
	if got := interp.GetGlobal("api_resp").Table().RawGetString("body").Str(); got != `{"body":"hello","id":1,"title":"seed"}` {
		t.Fatalf("api body = %q, want SQLite JSON row", got)
	}
	if got := interp.GetGlobal("form_resp").Table().RawGetString("body").Str(); got != `{"body":"form body","id":2,"title":"form title"}` {
		t.Fatalf("form body = %q, want inserted form row", got)
	}
	if got := interp.GetGlobal("form_err"); !got.IsNil() {
		t.Fatalf("form_err = %v, want nil", got)
	}
	if got := interp.GetGlobal("count_row").Table().RawGetString("n").Int(); got != 2 {
		t.Fatalf("count_row.n = %d, want 2", got)
	}
	for _, name := range []string{"closed", "waited"} {
		if got := interp.GetGlobal(name); !got.IsBool() || !got.Bool() {
			t.Fatalf("%s = %v, want true", name, got)
		}
	}
	for _, name := range []string{"close_err", "wait_err"} {
		if got := interp.GetGlobal(name); !got.IsNil() {
			t.Fatalf("%s = %v, want nil", name, got)
		}
	}
}

func TestServeDialectRequiresNetworkCapabilityWhenListening(t *testing.T) {
	interp := New()
	dialectModule := TableValue(BuildDialect(HostOptions{
		Call:           interp.CallFunction,
		NetworkAllowed: func() bool { return false },
		MaxHostResult:  interp.MaxHostResultBytes,
	}, interp.MaxHostResultBytes))
	interp.SetGlobal("dialect", dialectModule)
	interp.SetModule("dialect", dialectModule)

	src := `
		ok, err := pcall(func() {
			return serve {
				listen: "127.0.0.1:0"
				routes: {
					{method: "GET", path: "/", handler: func(req) { return "blocked" }},
				}
			}
		})
		router_only := serve {
			routes: {
				{method: "GET", path: "/", handler: func(req) { return "ok" }},
			}
		}
	`
	if _, err := interp.ExecString(src); err != nil {
		t.Fatalf("ExecString: %v", err)
	}
	if interp.GetGlobal("ok").Bool() {
		t.Fatalf("serve listen succeeded with network disabled, want pcall false")
	}
	if got := interp.GetGlobal("err").Str(); !strings.Contains(got, "network access disabled") {
		t.Fatalf("serve listen err = %q, want network access disabled", got)
	}
	if got := interp.GetGlobal("router_only"); !got.IsTable() {
		t.Fatalf("router_only = %v, want router table without listen", got)
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

func TestServeAppSupportsStandardHTTPMethods(t *testing.T) {
	interp := New()
	hostOpts := HostOptions{
		Call:           interp.CallFunction,
		NetworkAllowed: interp.NetworkAccessEnabled,
		MaxHostResult:  interp.MaxHostResultBytes,
	}
	httpModule := TableValue(BuildHTTP(hostOpts))
	netModule := TableValue(BuildNet(hostOpts))
	serveModule := TableValue(BuildServe(hostOpts))
	interp.SetGlobal("http", httpModule)
	interp.SetModule("http", httpModule)
	interp.SetGlobal("net", netModule)
	interp.SetModule("net", netModule)
	interp.SetGlobal("serve", serveModule)
	interp.SetModule("serve", serveModule)

	src := `
		server := serve.app({
			listen: "127.0.0.1:0",
			routes: {
				{method: "GET", path: "/items/:id", handler: func(req) {
					return "<html><body>item:" .. req.params.id .. "</body></html>"
				}},
				{method: "PUT", path: "/items/:id", handler: func(req) {
					data, err := req.json()
					assert(err == nil)
					return {id: req.params.id, name: data.name, method: req.method}
				}},
				{method: "DELETE", path: "/items/:id", handler: func(req) {
					return {id: req.params.id, deleted: true, method: req.method}
				}},
			},
		})
		htmlResp := http.get(server.url .. "/items/a-1")
		putResp, putErr := net.put(server.url .. "/items/a-1", "{\"name\":\"alpha\"}", {headers: {["Content-Type"]: "application/json"}})
		deleteResp, deleteErr := net.delete(server.url .. "/items/a-1")
		postResp, postErr := net.post(server.url .. "/items/a-1", "")
		closed, closeErr := server.close()
		waited, waitErr := server.wait()
	`
	if _, err := interp.ExecString(src); err != nil {
		t.Fatalf("ExecString: %v", err)
	}
	if got := interp.GetGlobal("htmlResp").Table().RawGetString("headers").Table().RawGetString("Content-Type").Str(); got != "text/html; charset=utf-8" {
		t.Fatalf("html content-type = %q, want text/html", got)
	}
	if got := interp.GetGlobal("putResp").Table().RawGetString("body").Str(); got != `{"id":"a-1","method":"PUT","name":"alpha"}` {
		t.Fatalf("put body = %q, want JSON update", got)
	}
	if got := interp.GetGlobal("deleteResp").Table().RawGetString("body").Str(); got != `{"deleted":true,"id":"a-1","method":"DELETE"}` {
		t.Fatalf("delete body = %q, want JSON delete", got)
	}
	if got := interp.GetGlobal("postResp").Table().RawGetString("status").Int(); got != 405 {
		t.Fatalf("post status = %d, want 405", got)
	}
	for _, name := range []string{"putErr", "deleteErr", "postErr", "closeErr", "waitErr"} {
		if got := interp.GetGlobal(name); !got.IsNil() {
			t.Fatalf("%s = %v, want nil", name, got)
		}
	}
	for _, name := range []string{"closed", "waited"} {
		if got := interp.GetGlobal(name); !got.IsBool() || !got.Bool() {
			t.Fatalf("%s = %v, want true", name, got)
		}
	}
}
