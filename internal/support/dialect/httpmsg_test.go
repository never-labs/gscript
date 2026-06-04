package dialect

import "testing"

func TestHTTPMessageParseRequestAndEncodeResponse(t *testing.T) {
	req, err := ParseHTTPMessage("GET /search?q=leia HTTP/1.1\r\nHost: example.test\r\nX-Trace: a\r\nX-Trace: b\r\n\r\nbody")
	if err != nil {
		t.Fatalf("ParseHTTPMessage request: %v", err)
	}
	if req.Type != "request" || req.Method != "GET" || req.Target != "/search?q=leia" || req.Version != "HTTP/1.1" {
		t.Fatalf("request start fields = %#v", req)
	}
	if got := req.Headers["X-Trace"]; len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("X-Trace headers = %#v", got)
	}
	if req.Body != "body" {
		t.Fatalf("body = %q, want body", req.Body)
	}

	encoded, err := EncodeHTTPMessage(HTTPMessage{
		Type:       "response",
		Version:    "HTTP/1.1",
		StatusCode: 204,
		Reason:     "No Content",
		Headers:    map[string][]string{"x-trace": {"a"}},
	})
	if err != nil {
		t.Fatalf("EncodeHTTPMessage response: %v", err)
	}
	want := "HTTP/1.1 204 No Content\r\nX-Trace: a\r\n\r\n"
	if encoded != want {
		t.Fatalf("encoded response = %q, want %q", encoded, want)
	}
}

func TestHTTPMessageRejectsInvalidShapes(t *testing.T) {
	for _, src := range []string{
		"",
		"GET /missing-version",
		"HTTP/1.1 nope",
		"GET / HTTP/1.1\r\nbad header\r\n\r\n",
	} {
		if _, err := ParseHTTPMessage(src); err == nil {
			t.Fatalf("ParseHTTPMessage(%q) succeeded, want error", src)
		}
	}
	if _, err := EncodeHTTPMessage(HTTPMessage{Type: "response", StatusCode: 42}); err == nil {
		t.Fatal("EncodeHTTPMessage invalid response status succeeded")
	}
	if _, err := EncodeHTTPMessage(HTTPMessage{Method: "GET", Target: "/", Headers: map[string][]string{"bad name": {"x"}}}); err == nil {
		t.Fatal("EncodeHTTPMessage invalid header name succeeded")
	}
}
