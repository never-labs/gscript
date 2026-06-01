package leia

import (
	"fmt"
	"reflect"
	"unicode"
	"unicode/utf8"
)

// Module is a Go-backed namespace exposed to Leia through require(name).
// Values use the same reflection conversion rules as RegisterFunc and Set.
type Module map[string]interface{}

type moduleFromOptions struct {
	nameMapper func(string) string
}

// ModuleFromOption configures ModuleFrom and RegisterModuleFrom.
type ModuleFromOption func(*moduleFromOptions)

// WithModuleNameMapper maps exported Go field and method names to script names.
// The default mapper lower-cases the first rune, so ToUpper becomes toUpper.
func WithModuleNameMapper(mapper func(string) string) ModuleFromOption {
	return func(opts *moduleFromOptions) {
		opts.nameMapper = mapper
	}
}

// WithModuleExactNames keeps exported Go field and method names unchanged.
func WithModuleExactNames() ModuleFromOption {
	return WithModuleNameMapper(func(name string) string { return name })
}

// ModuleFrom builds a require-able Module from a Go value. It is intended for
// service structs and third-party clients whose exported methods should be
// exposed to scripts without manually writing a Module literal.
func ModuleFrom(source interface{}, opts ...ModuleFromOption) (Module, error) {
	cfg := moduleFromOptions{nameMapper: lowerCamelExportedName}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.nameMapper == nil {
		cfg.nameMapper = func(name string) string { return name }
	}
	if source == nil {
		return nil, fmt.Errorf("ModuleFrom: nil source")
	}
	if existing, ok := source.(Module); ok {
		out := make(Module, len(existing))
		for k, v := range existing {
			out[k] = v
		}
		return out, nil
	}

	rv := reflect.ValueOf(source)
	if rv.Kind() == reflect.Map {
		return moduleFromMap(rv)
	}
	return moduleFromStructOrService(rv, cfg.nameMapper)
}

func moduleFromMap(rv reflect.Value) (Module, error) {
	if rv.Type().Key().Kind() != reflect.String {
		return nil, fmt.Errorf("ModuleFrom: map key must be string, got %s", rv.Type().Key())
	}
	out := make(Module, rv.Len())
	for _, key := range rv.MapKeys() {
		out[key.String()] = rv.MapIndex(key).Interface()
	}
	return out, nil
}

func moduleFromStructOrService(rv reflect.Value, mapper func(string) string) (Module, error) {
	if rv.Kind() == reflect.Pointer && rv.IsNil() {
		return nil, fmt.Errorf("ModuleFrom: nil %s", rv.Type())
	}
	out := make(Module)
	for i := 0; i < rv.NumMethod(); i++ {
		method := rv.Type().Method(i)
		if method.PkgPath != "" {
			continue
		}
		name := mapper(method.Name)
		if name == "" {
			continue
		}
		if _, exists := out[name]; exists {
			return nil, fmt.Errorf("ModuleFrom: duplicate member %q", name)
		}
		out[name] = rv.Method(i).Interface()
	}

	fields := rv
	for fields.Kind() == reflect.Pointer {
		if fields.IsNil() {
			return out, nil
		}
		fields = fields.Elem()
	}
	if fields.Kind() != reflect.Struct {
		if len(out) == 0 {
			return nil, fmt.Errorf("ModuleFrom: expected struct, pointer, map, or Module, got %s", rv.Type())
		}
		return out, nil
	}
	for i := 0; i < fields.NumField(); i++ {
		field := fields.Type().Field(i)
		if field.PkgPath != "" {
			continue
		}
		name := mapper(field.Name)
		if name == "" {
			continue
		}
		if _, exists := out[name]; exists {
			return nil, fmt.Errorf("ModuleFrom: duplicate member %q", name)
		}
		out[name] = fields.Field(i).Interface()
	}
	return out, nil
}

func lowerCamelExportedName(name string) string {
	if name == "" {
		return ""
	}
	r, size := utf8.DecodeRuneInString(name)
	if r == utf8.RuneError && size == 0 {
		return ""
	}
	return string(unicode.ToLower(r)) + name[size:]
}
