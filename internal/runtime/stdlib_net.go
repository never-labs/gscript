package runtime

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	hosthttp "github.com/never-labs/gscript/internal/stdlib/host/http"
)

// buildNetLib creates the "net" standard library table for HTTP client operations.
func buildNetLib(interps ...*Interpreter) *Table {
	t := NewTable()
	var interp *Interpreter
	if len(interps) > 0 {
		interp = interps[0]
	}

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name: "net." + name,
			Fn:   fn,
		}))
	}

	// net.get(url [, opts]) -> response table or nil, errMsg
	set("get", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'net.get' (string expected)")
		}
		if interp != nil && !interp.networkAccess {
			return nil, fmt.Errorf("network access disabled")
		}
		url := args[0].Str()
		var opts Value
		if len(args) >= 2 {
			opts = args[1]
		}
		return netDoRequest(interp, "GET", url, "", opts)
	})

	// net.post(url, body [, opts]) -> response table or nil, errMsg
	set("post", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'net.post' (url and body expected)")
		}
		if interp != nil && !interp.networkAccess {
			return nil, fmt.Errorf("network access disabled")
		}
		url := args[0].Str()
		body := args[1].Str()
		var opts Value
		if len(args) >= 3 {
			opts = args[2]
		}
		return netDoRequest(interp, "POST", url, body, opts)
	})

	// net.put(url, body [, opts]) -> response table or nil, errMsg
	set("put", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'net.put' (url and body expected)")
		}
		if interp != nil && !interp.networkAccess {
			return nil, fmt.Errorf("network access disabled")
		}
		url := args[0].Str()
		body := args[1].Str()
		var opts Value
		if len(args) >= 3 {
			opts = args[2]
		}
		return netDoRequest(interp, "PUT", url, body, opts)
	})

	// net.delete(url [, opts]) -> response table or nil, errMsg
	set("delete", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'net.delete' (string expected)")
		}
		if interp != nil && !interp.networkAccess {
			return nil, fmt.Errorf("network access disabled")
		}
		url := args[0].Str()
		var opts Value
		if len(args) >= 2 {
			opts = args[1]
		}
		return netDoRequest(interp, "DELETE", url, "", opts)
	})

	// net.patch(url, body [, opts]) -> response table or nil, errMsg
	set("patch", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'net.patch' (url and body expected)")
		}
		if interp != nil && !interp.networkAccess {
			return nil, fmt.Errorf("network access disabled")
		}
		url := args[0].Str()
		body := args[1].Str()
		var opts Value
		if len(args) >= 3 {
			opts = args[2]
		}
		return netDoRequest(interp, "PATCH", url, body, opts)
	})

	// net.request(opts_table) -> response table or nil, errMsg
	set("request", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'net.request' (table expected)")
		}
		if interp != nil && !interp.networkAccess {
			return nil, fmt.Errorf("network access disabled")
		}
		optsTable := args[0].Table()

		method := "GET"
		methodVal := optsTable.RawGet(StringValue("method"))
		if methodVal.IsString() && methodVal.Str() != "" {
			method = strings.ToUpper(methodVal.Str())
		}

		urlVal := optsTable.RawGet(StringValue("url"))
		if !urlVal.IsString() || urlVal.Str() == "" {
			return nil, fmt.Errorf("net.request: 'url' field is required")
		}
		url := urlVal.Str()

		body := ""
		bodyVal := optsTable.RawGet(StringValue("body"))
		if bodyVal.IsString() {
			body = bodyVal.Str()
		}

		return netDoRequest(interp, method, url, body, args[0])
	})

	return t
}

// netDoRequest performs an HTTP request and returns a GScript response table.
func netDoRequest(interp *Interpreter, method, url, body string, opts Value) ([]Value, error) {
	// Build the request
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	requestOpts := hosthttp.DefaultRequestOptions()

	// Parse opts if provided
	if opts.IsTable() {
		optsTable := opts.Table()

		// Headers
		headersVal := optsTable.RawGet(StringValue("headers"))
		if headersVal.IsTable() {
			requestOpts.Headers = make(map[string]string)
			hdrTable := headersVal.Table()
			key := NilValue()
			for {
				k, v, ok := hdrTable.Next(key)
				if !ok {
					break
				}
				if k.IsString() && v.IsString() {
					requestOpts.Headers[k.Str()] = v.Str()
				}
				key = k
			}
		}

		// Timeout
		timeoutVal := optsTable.RawGet(StringValue("timeout"))
		if timeoutVal.IsFloat() || timeoutVal.IsInt() {
			requestOpts.Timeout = time.Duration(toFloat(timeoutVal) * float64(time.Second))
		}

		// followRedirects
		followVal := optsTable.RawGet(StringValue("followRedirects"))
		if followVal.IsBool() {
			requestOpts.FollowRedirects = followVal.Bool()
		}
	}

	req, err := hosthttp.NewRequest(method, url, bodyReader, requestOpts)
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}

	// Execute request
	resp, err := hosthttp.NewClient(requestOpts).Do(req)
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	defer resp.Body.Close()

	// Read body
	var maxHostResult int64
	if interp != nil {
		maxHostResult = interp.maxHostResult
	}
	bodyBytes, err := ReadAllWithHostResultLimit(resp.Body, maxHostResult)
	if err != nil {
		return nil, err
	}
	bodyStr := string(bodyBytes)
	respInfo := hosthttp.ProjectResponse(resp)

	// Build response table
	result := NewTable()
	result.RawSet(StringValue("status"), IntValue(int64(respInfo.StatusCode)))
	result.RawSet(StringValue("statusText"), StringValue(respInfo.StatusText))
	result.RawSet(StringValue("body"), StringValue(bodyStr))
	result.RawSet(StringValue("ok"), BoolValue(respInfo.OK))

	// Response headers
	headers := NewTable()
	for k, v := range respInfo.Headers {
		headers.RawSet(StringValue(k), StringValue(v))
	}
	result.RawSet(StringValue("headers"), TableValue(headers))

	// json() method - parses body as JSON
	result.RawSet(StringValue("json"), FunctionValue(&GoFunction{
		Name: "response.json",
		Fn: func(args []Value) ([]Value, error) {
			var data interface{}
			if err := json.Unmarshal(bodyBytes, &data); err != nil {
				return []Value{NilValue(), StringValue(err.Error())}, nil
			}
			return []Value{goToGScript(data)}, nil
		},
	}))

	return []Value{TableValue(result), NilValue()}, nil
}
