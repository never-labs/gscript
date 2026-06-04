package bind

func registerDialectAI(register dialectRegisterFunc) {
	register([]string{"prompt"}, dialectHandler{
		eval:  func(body Value, options *Table) ([]Value, error) { return []Value{dialectPrompt(body, options)}, nil },
		block: func(body Value, options *Table) ([]Value, error) { return []Value{dialectPrompt(body, options)}, nil },
	})
	register([]string{"quote"}, dialectHandler{
		eval: func(body Value, options *Table) ([]Value, error) {
			return []Value{dialectQuote("quote", body, options)}, nil
		},
		block: func(body Value, options *Table) ([]Value, error) {
			return []Value{dialectQuote("quote", body, options)}, nil
		},
	})
}

func dialectPrompt(body Value, opts *Table) Value {
	out := NewTable()
	out.RawSetString("text", StringValue(body.String()))
	out.RawSetString("body", body)
	if opts != nil {
		out.RawSetString("options", TableValue(opts))
		if role := opts.RawGetString("role"); role.IsString() {
			out.RawSetString("role", role)
		}
	}
	return TableValue(out)
}

func dialectQuote(tag string, body Value, opts *Table) Value {
	out := NewTable()
	out.RawSetString("dialect", StringValue(tag))
	out.RawSetString("body", body)
	out.RawSetString("kind", StringValue(body.TypeName()))
	if opts != nil {
		out.RawSetString("options", TableValue(opts))
	}
	return TableValue(out)
}
