package leia

import stdbind "github.com/never-labs/leia/internal/stdlib/bind"

// DialectHandler implements a host-provided tagged dialect.
//
// The body is the tagged string or tagged block value. Options is nil when the
// call site did not provide an options table. Return one or more Leia values in
// the same style as ordinary host functions.
type DialectHandler func(body Value, options Value) ([]Value, error)

// DialectOptions describes a host-provided dialect.
type DialectOptions struct {
	Aliases      []string
	Category     string
	Capabilities []string
	Block        DialectHandler
}

type hostDialectSpec struct {
	Name         string
	Aliases      []string
	Category     string
	Capabilities []string
	Eval         DialectHandler
	Block        DialectHandler
}

// WithDialect registers a host-provided dialect for this VM.
//
// Built-in dialect names cannot be overridden. The handler runs for tagged
// strings, explicit dialect.eval calls, and tagged blocks unless opts.Block is
// supplied.
func WithDialect(name string, handler DialectHandler, opts ...DialectOptions) Option {
	return func(o *vmOptions) {
		spec := hostDialectSpec{Name: name, Eval: handler}
		if len(opts) > 0 {
			spec.Aliases = append([]string(nil), opts[0].Aliases...)
			spec.Category = opts[0].Category
			spec.Capabilities = append([]string(nil), opts[0].Capabilities...)
			spec.Block = opts[0].Block
		}
		o.dialects = append(o.dialects, spec)
	}
}

func bindDialectSpecs(specs []hostDialectSpec) []stdbind.DialectSpec {
	if len(specs) == 0 {
		return nil
	}
	out := make([]stdbind.DialectSpec, 0, len(specs))
	for _, spec := range specs {
		out = append(out, stdbind.DialectSpec{
			Name:         spec.Name,
			Aliases:      append([]string(nil), spec.Aliases...),
			Category:     spec.Category,
			Capabilities: append([]string(nil), spec.Capabilities...),
			Eval:         bindDialectHandler(spec.Eval),
			Block:        bindDialectHandler(spec.Block),
		})
	}
	return out
}

func bindDialectHandler(handler DialectHandler) func(stdbind.Value, *stdbind.Table) ([]stdbind.Value, error) {
	if handler == nil {
		return nil
	}
	return func(body stdbind.Value, options *stdbind.Table) ([]stdbind.Value, error) {
		optValue := Nil()
		if options != nil {
			optValue = toPublic(stdbind.TableValue(options))
		}
		values, err := handler(toPublic(body), optValue)
		if err != nil {
			return nil, err
		}
		out := make([]stdbind.Value, 0, len(values))
		for _, value := range values {
			out = append(out, fromPublic(value))
		}
		return out, nil
	}
}
