package bind

import (
	"context"
	"encoding/json"
	"fmt"
	hosthttp "github.com/never-labs/leia/internal/stdlib/lib/http"
	hostnet "github.com/never-labs/leia/internal/stdlib/lib/net"
	"net"
	"net/http"
	"strings"
	"sync"
)

func BuildHTTP(opts HostOptions) *Table {
	return BuildHTTPWithCallerAndPolicy(opts.Call, opts.NetworkAllowed, opts.MaxHostResult)
}

func BuildHTTPWithCaller(call ScriptFunctionCaller) *Table {
	return BuildHTTPWithCallerAndPolicy(call, nil, nil)
}

func BuildHTTPWithCallerAndNetworkPolicy(call ScriptFunctionCaller, networkAllowed func() bool) *Table {
	return BuildHTTPWithCallerAndPolicy(call, networkAllowed, nil)
}

func BuildHTTPWithCallerAndPolicy(call ScriptFunctionCaller, networkAllowed func() bool, maxHostResult func() int64) *Table {
	t := markStdlibBoundModule(NewTable())
	var handlerMu sync.Mutex
	maxResult := func() int64 {
		return hostResultLimit(maxHostResult)
	}

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name: "http." + name,
			Fn:   fn,
		}))
	}

	// http.listen(addr, handler [, options])
	// handler is called with (req, res) for each request
	// This BLOCKS until the server stops unless options.background is true.
	set("listen", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("http.listen requires address and handler")
		}
		if !HostBool(networkAllowed, true) {
			return nil, fmt.Errorf("network access disabled")
		}
		addr := args[0].Str()
		handler := args[1]
		background := httpListenBackground(args, 2)

		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			req, err := buildRequestTable(r, maxResult())
			if err != nil {
				http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
				return
			}
			// Build res table
			res, _ := buildResponseTableWithState(w, r)

			_, err = callHTTPHandler(call, &handlerMu, handler, req, res)
			if err != nil {
				http.Error(w, err.Error(), 500)
			}
		})

		return startHTTPServer(addr, mux, background)
	})

	// http.get(url) - simple HTTP GET client
	set("get", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("http.get requires a URL")
		}
		if !HostBool(networkAllowed, true) {
			return nil, fmt.Errorf("network access disabled")
		}
		url := args[0].Str()
		resp, err := http.Get(url)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		defer resp.Body.Close()
		body, err := ReadAllWithHostResultLimit(resp.Body, maxResult())
		if err != nil {
			return nil, err
		}

		result := NewTable()
		respInfo := hosthttp.ProjectResponse(resp)
		result.RawSet(StringValue("status"), IntValue(int64(respInfo.StatusCode)))
		result.RawSet(StringValue("body"), StringValue(string(body)))
		// Headers
		headers := NewTable()
		for k, v := range respInfo.Headers {
			headers.RawSet(StringValue(k), StringValue(v))
		}
		result.RawSet(StringValue("headers"), TableValue(headers))
		return []Value{TableValue(result)}, nil
	})

	// http.newRouter() - creates a router with route registration
	set("newRouter", func(args []Value) ([]Value, error) {
		return []Value{TableValue(buildRouterTable(call, maxResult))}, nil
	})

	return t
}

// buildRequestTable creates a Leia table representing an HTTP request.
func buildRequestTable(r *http.Request, maxHostResult int64) (Value, error) {
	t := NewTable()

	// Body
	body, err := ReadAllWithHostResultLimit(r.Body, maxHostResult)
	if err != nil {
		return NilValue(), err
	}
	reqInfo := hosthttp.ProjectRequest(r, body)

	t.RawSet(StringValue("method"), StringValue(reqInfo.Method))
	t.RawSet(StringValue("path"), StringValue(reqInfo.Path))
	t.RawSet(StringValue("url"), StringValue(reqInfo.URL))

	// Query params as table
	query := NewTable()
	for k, v := range reqInfo.Query {
		query.RawSet(StringValue(k), StringValue(v))
	}
	t.RawSet(StringValue("query"), TableValue(query))

	// Headers as table
	headers := NewTable()
	for k, v := range reqInfo.Headers {
		headers.RawSet(StringValue(k), StringValue(v))
	}
	t.RawSet(StringValue("headers"), TableValue(headers))

	t.RawSet(StringValue("body"), StringValue(string(reqInfo.Body)))
	t.RawSet(StringValue("params"), TableValue(NewTable()))

	// req.param(name) - get query param
	t.RawSet(StringValue("param"), FunctionValue(&GoFunction{
		Name: "req.param",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 {
				return []Value{NilValue()}, nil
			}
			val := r.URL.Query().Get(args[0].Str())
			if val == "" {
				return []Value{NilValue()}, nil
			}
			return []Value{StringValue(val)}, nil
		},
	}))

	// req.json() - parse body as JSON into a table
	t.RawSet(StringValue("json"), FunctionValue(&GoFunction{
		Name: "req.json",
		Fn: func(args []Value) ([]Value, error) {
			var data interface{}
			if err := json.Unmarshal(body, &data); err != nil {
				return []Value{NilValue(), StringValue(err.Error())}, nil
			}
			return []Value{goToLeia(data)}, nil
		},
	}))

	return TableValue(t), nil
}

// buildResponseTable creates a Leia table representing an HTTP response writer.
type httpResponseState struct {
	written   bool
	statusSet bool
}

func buildResponseTable(w http.ResponseWriter, r *http.Request) Value {
	res, _ := buildResponseTableWithState(w, r)
	return res
}

func buildResponseTableWithState(w http.ResponseWriter, r *http.Request) (Value, *httpResponseState) {
	t := NewTable()
	state := &httpResponseState{}

	// res.write(body) - write response body
	t.RawSet(StringValue("write"), FunctionValue(&GoFunction{
		Name: "res.write",
		Fn: func(args []Value) ([]Value, error) {
			if !state.statusSet {
				w.WriteHeader(200)
				state.statusSet = true
			}
			if len(args) > 0 {
				state.written = true
				fmt.Fprint(w, args[0].String())
			}
			return nil, nil
		},
	}))

	// res.writeln(body) - write with newline
	t.RawSet(StringValue("writeln"), FunctionValue(&GoFunction{
		Name: "res.writeln",
		Fn: func(args []Value) ([]Value, error) {
			if !state.statusSet {
				w.WriteHeader(200)
				state.statusSet = true
			}
			if len(args) > 0 {
				state.written = true
				fmt.Fprintln(w, args[0].String())
			}
			return nil, nil
		},
	}))

	// res.json(value) - write JSON response
	t.RawSet(StringValue("json"), FunctionValue(&GoFunction{
		Name: "res.json",
		Fn: func(args []Value) ([]Value, error) {
			w.Header().Set("Content-Type", "application/json")
			if !state.statusSet {
				w.WriteHeader(200)
				state.statusSet = true
			}
			if len(args) > 0 {
				data := leiaToGo(args[0])
				jsonBytes, err := json.Marshal(data)
				if err != nil {
					return nil, err
				}
				state.written = true
				w.Write(jsonBytes)
			}
			return nil, nil
		},
	}))

	// res.status(code) - set status code (must call before write)
	t.RawSet(StringValue("status"), FunctionValue(&GoFunction{
		Name: "res.status",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) > 0 && !state.written {
				code := int(args[0].Int())
				w.WriteHeader(code)
				state.statusSet = true
			}
			return []Value{TableValue(t)}, nil // return res for chaining
		},
	}))

	// res.header(key, value) - set response header
	t.RawSet(StringValue("header"), FunctionValue(&GoFunction{
		Name: "res.header",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) >= 2 {
				w.Header().Set(args[0].Str(), args[1].Str())
			}
			return []Value{TableValue(t)}, nil // return res for chaining
		},
	}))

	// res.redirect(url [, code]) - redirect
	t.RawSet(StringValue("redirect"), FunctionValue(&GoFunction{
		Name: "res.redirect",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 {
				return nil, nil
			}
			code := 302
			if len(args) >= 2 {
				code = int(args[1].Int())
			}
			http.Redirect(w, r, args[0].Str(), code)
			state.written = true
			state.statusSet = true
			return nil, nil
		},
	}))

	return TableValue(t), state
}

// buildRouterTable creates a router with route registration.
func buildRouterTable(call ScriptFunctionCaller, maxHostResult func() int64) *Table {
	return newHTTPRouter(call, maxHostResult, httpRouterOptions{}).table
}

type httpRouterOptions struct {
	autoRespond bool
}

type httpRouteEntry struct {
	method  string
	pattern string
	handler Value
}

type httpRouter struct {
	table  *Table
	mu     sync.RWMutex
	routes []httpRouteEntry
}

func newHTTPRouter(call ScriptFunctionCaller, maxHostResult func() int64, opts httpRouterOptions) *httpRouter {
	t := NewTable()
	var handlerMu sync.Mutex
	hostResultLimit := func() int64 {
		if maxHostResult == nil {
			return 0
		}
		return maxHostResult()
	}
	router := &httpRouter{table: t}

	registerRoute := func(method, pattern string, handler Value) {
		router.addRoute(method, pattern, handler)
	}

	registerMethod := func(name, method string) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name: "router." + name,
			Fn: func(args []Value) ([]Value, error) {
				if len(args) >= 2 {
					registerRoute(method, args[0].Str(), args[1])
				}
				return []Value{TableValue(t)}, nil
			},
		}))
	}

	registerMethod("get", http.MethodGet)
	registerMethod("head", http.MethodHead)
	registerMethod("post", http.MethodPost)
	registerMethod("put", http.MethodPut)
	registerMethod("patch", http.MethodPatch)
	registerMethod("delete", http.MethodDelete)
	registerMethod("options", http.MethodOptions)

	// router.any(pattern, handler)
	t.RawSet(StringValue("any"), FunctionValue(&GoFunction{
		Name: "router.any",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) >= 2 {
				registerRoute("", args[0].Str(), args[1])
			}
			return []Value{TableValue(t)}, nil
		},
	}))

	// router.listen(addr [, options])
	t.RawSet(StringValue("listen"), FunctionValue(&GoFunction{
		Name: "router.listen",
		Fn: func(args []Value) ([]Value, error) {
			addr := ":8080"
			if len(args) >= 1 {
				addr = args[0].Str()
			}
			background := httpListenBackground(args, 1)
			return startHTTPServer(addr, router.handler(call, &handlerMu, hostResultLimit, opts), background)
		},
	}))

	return router
}

func (router *httpRouter) addRoute(method, pattern string, handler Value) {
	router.mu.Lock()
	router.routes = append(router.routes, httpRouteEntry{method: method, pattern: pattern, handler: handler})
	router.mu.Unlock()
}

func (router *httpRouter) handler(call ScriptFunctionCaller, handlerMu *sync.Mutex, hostResultLimit func() int64, opts httpRouterOptions) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route, params, methodAllowed := router.match(r.Method, r.URL.Path)
		if route == nil {
			if methodAllowed {
				http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
				return
			}
			http.NotFound(w, r)
			return
		}
		req, err := buildRequestTable(r, hostResultLimit())
		if err != nil {
			http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		setRequestParams(req, params)
		res, state := buildResponseTableWithState(w, r)
		values, err := callHTTPHandler(call, handlerMu, route.handler, req, res)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if opts.autoRespond {
			if err := writeHTTPAutoResponse(w, state, values); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}
	})
}

func (router *httpRouter) match(method, path string) (*httpRouteEntry, map[string]string, bool) {
	router.mu.RLock()
	defer router.mu.RUnlock()
	methodAllowed := false
	for i := range router.routes {
		route := &router.routes[i]
		params, ok := matchHTTPRoutePath(route.pattern, path)
		if !ok {
			continue
		}
		if route.method == "" || route.method == method {
			return route, params, false
		}
		methodAllowed = true
	}
	return nil, nil, methodAllowed
}

func matchHTTPRoutePath(pattern, path string) (map[string]string, bool) {
	if pattern == path && !strings.Contains(pattern, ":") {
		return nil, true
	}
	pSegs := splitHTTPRoutePath(pattern)
	pathSegs := splitHTTPRoutePath(path)
	if len(pSegs) != len(pathSegs) {
		return nil, false
	}
	var params map[string]string
	for i, pseg := range pSegs {
		if strings.HasPrefix(pseg, ":") && len(pseg) > 1 {
			if params == nil {
				params = make(map[string]string)
			}
			params[pseg[1:]] = pathSegs[i]
			continue
		}
		if pseg != pathSegs[i] {
			return nil, false
		}
	}
	return params, true
}

func splitHTTPRoutePath(path string) []string {
	if path == "/" {
		return nil
	}
	return strings.Split(strings.Trim(path, "/"), "/")
}

func setRequestParams(req Value, params map[string]string) {
	if !req.IsTable() {
		return
	}
	t := NewTable()
	for key, val := range params {
		t.RawSetString(key, StringValue(val))
	}
	req.Table().RawSetString("params", TableValue(t))
}

func writeHTTPAutoResponse(w http.ResponseWriter, state *httpResponseState, values []Value) error {
	if state == nil || state.written || len(values) == 0 || values[0].IsNil() {
		return nil
	}
	value := values[0]
	if value.IsTable() {
		w.Header().Set("Content-Type", "application/json")
		data, err := json.Marshal(leiaToGo(value))
		if err != nil {
			return err
		}
		if !state.statusSet {
			w.WriteHeader(http.StatusOK)
			state.statusSet = true
		}
		state.written = true
		_, err = w.Write(data)
		return err
	}
	body := value.String()
	if w.Header().Get("Content-Type") == "" {
		if looksLikeHTML(body) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
		} else {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		}
	}
	if !state.statusSet {
		w.WriteHeader(http.StatusOK)
		state.statusSet = true
	}
	state.written = true
	_, err := fmt.Fprint(w, body)
	return err
}

func looksLikeHTML(body string) bool {
	trimmed := strings.TrimSpace(body)
	return strings.HasPrefix(trimmed, "<!doctype html") ||
		strings.HasPrefix(trimmed, "<html") ||
		strings.HasPrefix(trimmed, "<")
}

type httpServerHandle struct {
	server *http.Server
	ln     net.Listener
	done   chan error

	mu     sync.Mutex
	closed bool
}

func httpListenBackground(args []Value, optIndex int) bool {
	if len(args) <= optIndex || !args[optIndex].IsTable() {
		return false
	}
	return args[optIndex].Table().RawGetString("background").Truthy()
}

func startHTTPServer(addr string, handler http.Handler, background bool) ([]Value, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	server := &http.Server{Handler: handler}
	handle := &httpServerHandle{
		server: server,
		ln:     ln,
		done:   make(chan error, 1),
	}

	if !background {
		fmt.Printf("Leia HTTP server listening on %s\n", ln.Addr().String())
		err := server.Serve(ln)
		if err == http.ErrServerClosed {
			return nil, nil
		}
		return nil, err
	}

	go func() {
		err := server.Serve(ln)
		if err == http.ErrServerClosed {
			err = nil
		}
		handle.done <- err
	}()

	return []Value{TableValue(buildHTTPServerHandleTable(handle))}, nil
}

func buildHTTPServerHandleTable(handle *httpServerHandle) *Table {
	t := NewTable()
	addr := handle.ln.Addr().String()
	urlAddr := httpConnectAddr(addr)
	t.RawSetString("addr", StringValue(addr))
	t.RawSetString("url", StringValue("http://"+urlAddr))

	t.RawSetString("close", FunctionValue(&GoFunction{
		Name: "http.server.close",
		Fn: func(args []Value) ([]Value, error) {
			err := handle.close()
			if err != nil {
				return []Value{NilValue(), StringValue(err.Error())}, nil
			}
			return []Value{BoolValue(true)}, nil
		},
	}))
	t.RawSetString("shutdown", FunctionValue(&GoFunction{
		Name: "http.server.shutdown",
		Fn: func(args []Value) ([]Value, error) {
			err := handle.shutdown()
			if err != nil {
				return []Value{NilValue(), StringValue(err.Error())}, nil
			}
			return []Value{BoolValue(true)}, nil
		},
	}))
	t.RawSetString("wait", FunctionValue(&GoFunction{
		Name: "http.server.wait",
		Fn: func(args []Value) ([]Value, error) {
			err := <-handle.done
			if err != nil {
				return []Value{NilValue(), StringValue(err.Error())}, nil
			}
			return []Value{BoolValue(true)}, nil
		},
	}))
	return t
}

func callHTTPHandler(call ScriptFunctionCaller, mu *sync.Mutex, handler Value, req, res Value) ([]Value, error) {
	mu.Lock()
	defer mu.Unlock()
	return call(handler, []Value{req, res})
}

func httpConnectAddr(addr string) string {
	return hostnet.ConnectAddr(addr)
}

func (h *httpServerHandle) close() error {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return nil
	}
	h.closed = true
	h.mu.Unlock()
	return h.server.Close()
}

func (h *httpServerHandle) shutdown() error {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return nil
	}
	h.closed = true
	h.mu.Unlock()
	return h.server.Shutdown(context.Background())
}

// goToLeia converts Go values (from JSON unmarshal) to Leia Values.
func goToLeia(v interface{}) Value {
	switch val := v.(type) {
	case nil:
		return NilValue()
	case bool:
		return BoolValue(val)
	case float64:
		return FloatValue(val)
	case string:
		return StringValue(val)
	case []interface{}:
		t := NewTable()
		for i, item := range val {
			t.RawSet(IntValue(int64(i+1)), goToLeia(item))
		}
		return TableValue(t)
	case map[string]interface{}:
		t := NewTable()
		for k, item := range val {
			t.RawSet(StringValue(k), goToLeia(item))
		}
		return TableValue(t)
	default:
		return StringValue(fmt.Sprintf("%v", val))
	}
}

// leiaToGo converts Leia Values to Go values (for JSON marshal).
func leiaToGo(v Value) interface{} {
	switch v.Type() {
	case TypeNil:
		return nil
	case TypeBool:
		return v.Bool()
	case TypeInt:
		return v.Int()
	case TypeFloat:
		return v.Number()
	case TypeString:
		return v.Str()
	case TypeTable:
		t := v.Table()
		// Check if it's array-like
		length := t.Length()
		if length > 0 {
			arr := make([]interface{}, length)
			for i := 1; i <= length; i++ {
				arr[i-1] = leiaToGo(t.RawGet(IntValue(int64(i))))
			}
			return arr
		}
		// Hash map
		m := make(map[string]interface{})
		key := NilValue()
		for {
			k, val, ok := t.Next(key)
			if !ok {
				break
			}
			m[k.String()] = leiaToGo(val)
			key = k
		}
		return m
	default:
		return v.String()
	}
}
