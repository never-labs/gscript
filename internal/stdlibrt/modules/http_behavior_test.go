package modules

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPBuildRequestTable(t *testing.T) {
	// Create a fake HTTP request
	req := httptest.NewRequest("GET", "/hello?name=world&foo=bar", nil)
	req.Header.Set("Content-Type", "text/plain")

	val, err := buildRequestTable(req, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !val.IsTable() {
		t.Fatalf("expected table, got %s", val.TypeName())
	}
	tbl := val.Table()

	// Check method
	method := tbl.RawGet(StringValue("method"))
	if method.Str() != "GET" {
		t.Errorf("expected method=GET, got %s", method.Str())
	}

	// Check path
	path := tbl.RawGet(StringValue("path"))
	if path.Str() != "/hello" {
		t.Errorf("expected path=/hello, got %s", path.Str())
	}

	// Check query params table
	query := tbl.RawGet(StringValue("query"))
	if !query.IsTable() {
		t.Fatalf("expected query to be table, got %s", query.TypeName())
	}
	nameParam := query.Table().RawGet(StringValue("name"))
	if nameParam.Str() != "world" {
		t.Errorf("expected query.name=world, got %s", nameParam.Str())
	}

	// Check param function
	paramFn := tbl.RawGet(StringValue("param"))
	if !paramFn.IsFunction() {
		t.Fatalf("expected param to be function, got %s", paramFn.TypeName())
	}
	results, err := paramFn.GoFunction().Fn([]Value{StringValue("name")})
	if err != nil {
		t.Fatalf("param() error: %v", err)
	}
	if len(results) == 0 || results[0].Str() != "world" {
		t.Errorf("expected param('name')='world', got %v", results)
	}

	// Check param for missing key
	results, _ = paramFn.GoFunction().Fn([]Value{StringValue("missing")})
	if len(results) == 0 || !results[0].IsNil() {
		t.Errorf("expected param('missing')=nil, got %v", results)
	}
}

func TestHTTPBuildResponseTable(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	val := buildResponseTable(w, req)
	if !val.IsTable() {
		t.Fatalf("expected table, got %s", val.TypeName())
	}
	tbl := val.Table()

	// Test res.header
	headerFn := tbl.RawGet(StringValue("header")).GoFunction()
	headerFn.Fn([]Value{StringValue("X-Custom"), StringValue("test-value")})

	// Test res.write
	writeFn := tbl.RawGet(StringValue("write")).GoFunction()
	writeFn.Fn([]Value{StringValue("hello world")})

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", string(body))
	}
	if resp.Header.Get("X-Custom") != "test-value" {
		t.Errorf("expected X-Custom header, got %v", resp.Header)
	}
}

func TestHTTPResponseJSON(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	val := buildResponseTable(w, req)
	tbl := val.Table()

	// Build a GScript table to serialize
	data := NewTable()
	data.RawSet(StringValue("name"), StringValue("GScript"))
	data.RawSet(StringValue("version"), IntValue(1))

	jsonFn := tbl.RawGet(StringValue("json")).GoFunction()
	_, err := jsonFn.Fn([]Value{TableValue(data)})
	if err != nil {
		t.Fatalf("res.json() error: %v", err)
	}

	resp := w.Result()
	if resp.Header.Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type: application/json, got %s", resp.Header.Get("Content-Type"))
	}

	body, _ := io.ReadAll(resp.Body)
	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}
	if parsed["name"] != "GScript" {
		t.Errorf("expected name=GScript, got %v", parsed["name"])
	}
}

func TestHTTPResponseStatus(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	val := buildResponseTable(w, req)
	tbl := val.Table()

	statusFn := tbl.RawGet(StringValue("status")).GoFunction()
	results, _ := statusFn.Fn([]Value{IntValue(404)})

	// Should return the res table for chaining
	if len(results) == 0 || !results[0].IsTable() {
		t.Errorf("expected status() to return table for chaining")
	}

	writeFn := tbl.RawGet(StringValue("write")).GoFunction()
	writeFn.Fn([]Value{StringValue("not found")})

	resp := w.Result()
	if resp.StatusCode != 404 {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}
}

func TestHTTPEndToEnd(t *testing.T) {
	// Create an interpreter and use http library to handle a request
	interp := New()

	// Get the buildResponseTable and buildRequestTable via a real handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqVal, err := buildRequestTable(r, 0)
		if err != nil {
			t.Fatal(err)
		}
		resVal := buildResponseTable(w, r)

		// Simulate calling: res.write("Hello " .. req.method)
		reqTbl := reqVal.Table()
		resTbl := resVal.Table()

		method := reqTbl.RawGet(StringValue("method"))
		writeFn := resTbl.RawGet(StringValue("write")).GoFunction()
		writeFn.Fn([]Value{StringValue("Hello " + method.Str())})
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	resp, err := http.Get(server.URL + "/test")
	if err != nil {
		t.Fatalf("HTTP GET failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "Hello GET" {
		t.Errorf("expected 'Hello GET', got '%s'", string(body))
	}

	_ = interp // used to ensure the test is in the right package
}

func TestHTTPEndToEndWithInterpreter(t *testing.T) {
	// Test that the interpreter can call a GScript handler function
	interp := New()

	// Create a GScript handler function as a GoFunction for simplicity
	handlerFn := FunctionValue(&GoFunction{
		Name: "testHandler",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 2 {
				return nil, nil
			}
			reqTbl := args[0].Table()
			resTbl := args[1].Table()

			path := reqTbl.RawGet(StringValue("path"))
			writeFn := resTbl.RawGet(StringValue("write")).GoFunction()
			writeFn.Fn([]Value{StringValue("Path: " + path.Str())})
			return nil, nil
		},
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, err := buildRequestTable(r, 0)
		if err != nil {
			t.Fatal(err)
		}
		res := buildResponseTable(w, r)
		_, _ = interp.CallFunction(handlerFn, []Value{req, res})
	}))
	defer server.Close()

	resp, err := http.Get(server.URL + "/mypath")
	if err != nil {
		t.Fatalf("HTTP GET failed: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "Path: /mypath" {
		t.Errorf("expected 'Path: /mypath', got '%s'", string(body))
	}
}

func TestHTTPListenBackgroundHandleRoundTrip(t *testing.T) {
	interp := New()
	httpModule := TableValue(BuildHTTP(HostOptions{Call: interp.CallFunction, NetworkAllowed: interp.NetworkAccessEnabled, MaxHostResult: interp.MaxHostResultBytes}))
	interp.SetGlobal("http", httpModule)
	interp.SetModule("http", httpModule)
	src := `
		server := http.listen("127.0.0.1:0", func(req, res) {
			res.write("ok:" .. req.path)
		}, {background: true})
		serverType := type(server)
		addrType := type(server.addr)
		urlType := type(server.url)
		closeType := type(server.close)
		resp := http.get(server.url .. "/health")
		status := resp.status
		body := resp.body
		closed, closeErr := server.close()
		waited, waitErr := server.wait()
	`
	if _, err := interp.ExecString(src); err != nil {
		t.Fatalf("ExecString: %v", err)
	}
	for name, want := range map[string]string{
		"serverType": "table",
		"addrType":   "string",
		"urlType":    "string",
		"closeType":  "function",
		"body":       "ok:/health",
	} {
		if got := interp.GetGlobal(name); got.Str() != want {
			t.Fatalf("%s = %v, want %q", name, got, want)
		}
	}
	if got := interp.GetGlobal("status"); !got.IsInt() || got.Int() != 200 {
		t.Fatalf("status = %v, want 200", got)
	}
	if got := interp.GetGlobal("closed"); !got.IsBool() || !got.Bool() {
		t.Fatalf("closed = %v, want true", got)
	}
	if got := interp.GetGlobal("waited"); !got.IsBool() || !got.Bool() {
		t.Fatalf("waited = %v, want true", got)
	}
	if got := interp.GetGlobal("closeErr"); !got.IsNil() {
		t.Fatalf("closeErr = %v, want nil", got)
	}
	if got := interp.GetGlobal("waitErr"); !got.IsNil() {
		t.Fatalf("waitErr = %v, want nil", got)
	}
}

func TestHTTPRouterBackgroundShutdown(t *testing.T) {
	interp := New()
	httpModule := TableValue(BuildHTTP(HostOptions{Call: interp.CallFunction, NetworkAllowed: interp.NetworkAccessEnabled, MaxHostResult: interp.MaxHostResultBytes}))
	interp.SetGlobal("http", httpModule)
	interp.SetModule("http", httpModule)
	src := `
		router := http.newRouter()
		router.get("/route", func(req, res) {
			res.status(201).write("route:" .. req.path)
		})
		server := router.listen("127.0.0.1:0", {background: true})
		resp := http.get(server.url .. "/route")
		status := resp.status
		body := resp.body
		shut, shutErr := server.shutdown()
		waited, waitErr := server.wait()
	`
	if _, err := interp.ExecString(src); err != nil {
		t.Fatalf("ExecString: %v", err)
	}
	if got := interp.GetGlobal("status"); !got.IsInt() || got.Int() != 201 {
		t.Fatalf("status = %v, want 201", got)
	}
	if got := interp.GetGlobal("body"); got.Str() != "route:/route" {
		t.Fatalf("body = %v, want route:/route", got)
	}
	if got := interp.GetGlobal("shut"); !got.IsBool() || !got.Bool() {
		t.Fatalf("shut = %v, want true", got)
	}
	if got := interp.GetGlobal("waited"); !got.IsBool() || !got.Bool() {
		t.Fatalf("waited = %v, want true", got)
	}
	if got := interp.GetGlobal("shutErr"); !got.IsNil() {
		t.Fatalf("shutErr = %v, want nil", got)
	}
	if got := interp.GetGlobal("waitErr"); !got.IsNil() {
		t.Fatalf("waitErr = %v, want nil", got)
	}
}

func TestHTTPListenBackgroundPortConflictFailsSynchronously(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen fixture: %v", err)
	}
	defer ln.Close()

	interp := New()
	httpModule := TableValue(BuildHTTP(HostOptions{Call: interp.CallFunction, NetworkAllowed: interp.NetworkAccessEnabled, MaxHostResult: interp.MaxHostResultBytes}))
	interp.SetGlobal("http", httpModule)
	interp.SetModule("http", httpModule)
	src := fmt.Sprintf(`
		ok, err := pcall(http.listen, "%s", func(req, res) {}, {background: true})
	`, ln.Addr().String())
	if _, err := interp.ExecString(src); err != nil {
		t.Fatalf("ExecString: %v", err)
	}
	if got := interp.GetGlobal("ok"); !got.IsBool() || got.Bool() {
		t.Fatalf("ok = %v, want false", got)
	}
	if got := interp.GetGlobal("err"); !got.IsString() || got.Str() == "" {
		t.Fatalf("err = %v, want non-empty string", got)
	}
}
