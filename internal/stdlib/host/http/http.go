package http

import (
	"io"
	stdhttp "net/http"
	"strings"
	"time"
)

// RequestOptions holds host-level HTTP client defaults independent of runtime
// Value/Table conversion.
type RequestOptions struct {
	Headers         map[string]string
	Timeout         time.Duration
	FollowRedirects bool
}

func DefaultRequestOptions() RequestOptions {
	return RequestOptions{
		Timeout:         30 * time.Second,
		FollowRedirects: true,
	}
}

func NewRequest(method, url string, body io.Reader, opts RequestOptions) (*stdhttp.Request, error) {
	req, err := stdhttp.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	for key, value := range opts.Headers {
		req.Header.Set(key, value)
	}
	return req, nil
}

func NewClient(opts RequestOptions) *stdhttp.Client {
	client := &stdhttp.Client{Timeout: opts.Timeout}
	if !opts.FollowRedirects {
		client.CheckRedirect = func(req *stdhttp.Request, via []*stdhttp.Request) error {
			return stdhttp.ErrUseLastResponse
		}
	}
	return client
}

type ResponseInfo struct {
	StatusCode int
	StatusText string
	OK         bool
	Headers    map[string]string
}

func ProjectResponse(resp *stdhttp.Response) ResponseInfo {
	return ResponseInfo{
		StatusCode: resp.StatusCode,
		StatusText: resp.Status,
		OK:         resp.StatusCode < 400,
		Headers:    ProjectHeader(resp.Header),
	}
}

func ProjectHeader(header stdhttp.Header) map[string]string {
	projected := make(map[string]string, len(header))
	for key, values := range header {
		projected[key] = strings.Join(values, ", ")
	}
	return projected
}

type RequestInfo struct {
	Method  string
	Path    string
	URL     string
	Query   map[string]string
	Headers map[string]string
	Body    []byte
}

func ProjectRequest(req *stdhttp.Request, body []byte) RequestInfo {
	return RequestInfo{
		Method:  req.Method,
		Path:    req.URL.Path,
		URL:     req.URL.String(),
		Query:   projectValues(req.URL.Query()),
		Headers: ProjectHeader(req.Header),
		Body:    body,
	}
}

func projectValues(values map[string][]string) map[string]string {
	projected := make(map[string]string, len(values))
	for key, vals := range values {
		projected[key] = strings.Join(vals, ", ")
	}
	return projected
}
