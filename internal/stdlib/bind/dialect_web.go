package bind

func registerDialectWeb(register dialectRegisterFunc, opts HostOptions, maxHostResult func() int64) {
	serveModule := BuildServeWithCallerAndPolicy(opts.Call, opts.NetworkAllowed, maxHostResult)
	serveApp := serveModule.RawGetString("app").GoFunction()
	register([]string{"serve"}, dialectHandler{
		eval: func(body Value, _ *Table) ([]Value, error) {
			return serveApp.Fn([]Value{body})
		},
		block: func(body Value, _ *Table) ([]Value, error) {
			return serveApp.Fn([]Value{body})
		},
	})
}
