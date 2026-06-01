package modules

import (
	"testing"

	"github.com/never-labs/leia/internal/stdlibrt"
)

func TestHTTPLibRegistered(t *testing.T) {
	interp := runWithLib(t, `
		result := type(http)
	`, "http", BuildHTTP(stdlibrt.HostOptions{}))
	v := interp.GetGlobal("result")
	if v.Str() != "table" {
		t.Errorf("expected http to be 'table', got %s", v.Str())
	}
}

func TestHTTPLibFunctions(t *testing.T) {
	interp := runWithLib(t, `
		a := type(http.listen)
		b := type(http.get)
		c := type(http.newRouter)
	`, "http", BuildHTTP(stdlibrt.HostOptions{}))
	if interp.GetGlobal("a").Str() != "function" {
		t.Errorf("expected http.listen to be 'function', got %s", interp.GetGlobal("a").Str())
	}
	if interp.GetGlobal("b").Str() != "function" {
		t.Errorf("expected http.get to be 'function', got %s", interp.GetGlobal("b").Str())
	}
	if interp.GetGlobal("c").Str() != "function" {
		t.Errorf("expected http.newRouter to be 'function', got %s", interp.GetGlobal("c").Str())
	}
}

func TestHTTPNewRouter(t *testing.T) {
	interp := runWithLib(t, `
		router := http.newRouter()
		result := type(router)
		has_get := type(router.get)
		has_post := type(router.post)
		has_any := type(router.any)
		has_listen := type(router.listen)
	`, "http", BuildHTTP(stdlibrt.HostOptions{}))
	if interp.GetGlobal("result").Str() != "table" {
		t.Errorf("expected router to be 'table', got %s", interp.GetGlobal("result").Str())
	}
	if interp.GetGlobal("has_get").Str() != "function" {
		t.Errorf("expected router.get to be 'function', got %s", interp.GetGlobal("has_get").Str())
	}
	if interp.GetGlobal("has_post").Str() != "function" {
		t.Errorf("expected router.post to be 'function', got %s", interp.GetGlobal("has_post").Str())
	}
	if interp.GetGlobal("has_any").Str() != "function" {
		t.Errorf("expected router.any to be 'function', got %s", interp.GetGlobal("has_any").Str())
	}
	if interp.GetGlobal("has_listen").Str() != "function" {
		t.Errorf("expected router.listen to be 'function', got %s", interp.GetGlobal("has_listen").Str())
	}
}

func TestHTTPRouterChaining(t *testing.T) {
	interp := runWithLib(t, `
		router := http.newRouter()
		r2 := router.get("/test", func(req, res) {})
		same := r2 == router
	`, "http", BuildHTTP(stdlibrt.HostOptions{}))
	if !interp.GetGlobal("same").Truthy() {
		t.Errorf("expected router.get to return the same router for chaining")
	}
}
