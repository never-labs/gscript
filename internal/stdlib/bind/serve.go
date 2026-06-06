package bind

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
)

func BuildServe(opts HostOptions) *Table {
	return BuildServeWithCallerAndPolicy(opts.Call, opts.NetworkAllowed, opts.MaxHostResult)
}

func BuildServeWithCallerAndPolicy(call ScriptFunctionCaller, networkAllowed func() bool, maxHostResult func() int64) *Table {
	t := markStdlibBoundModule(NewTable())
	maxResult := func() int64 {
		return hostResultLimit(maxHostResult)
	}
	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSetString(name, FunctionValue(&GoFunction{Name: "serve." + name, Fn: fn}))
	}

	set("app", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("serve.app requires a config table")
		}
		if call == nil {
			return nil, fmt.Errorf("serve.app requires script callback support")
		}
		config := args[0].Table()
		router := newHTTPRouter(call, maxResult, httpRouterOptions{autoRespond: true})
		routes := config.RawGetString("routes")
		if routes.IsTable() {
			for i := 1; i <= routes.Table().Length(); i++ {
				if err := serveRegisterRoute(router, routes.Table().RawGetInt(int64(i))); err != nil {
					return nil, err
				}
			}
		}
		listen := config.RawGetString("listen")
		if listen.IsNil() {
			return []Value{TableValue(router.table)}, nil
		}
		if !HostBool(networkAllowed, true) {
			return nil, fmt.Errorf("network access disabled")
		}
		var handlerMu sync.Mutex
		handler := router.handler(call, &handlerMu, maxResult, httpRouterOptions{autoRespond: true})
		return startHTTPServer(listen.Str(), handler, true)
	})

	return t
}

func serveRegisterRoute(router *httpRouter, value Value) error {
	if !value.IsTable() {
		return fmt.Errorf("serve route must be a table")
	}
	route := value.Table()
	method := strings.ToUpper(route.RawGetString("method").Str())
	if !serveRouteMethodAllowed(method) {
		return fmt.Errorf("serve route method must be a standard HTTP method")
	}
	path := route.RawGetString("path")
	if !path.IsString() || path.Str() == "" {
		return fmt.Errorf("serve route requires path")
	}
	handler := route.RawGetString("handler")
	if !handler.IsFunction() {
		return fmt.Errorf("serve route requires handler function")
	}
	return router.addRoute(method, path.Str(), handler)
}

func serveRouteMethodAllowed(method string) bool {
	switch method {
	case http.MethodGet,
		http.MethodHead,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodDelete,
		http.MethodOptions:
		return true
	default:
		return false
	}
}
