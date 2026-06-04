package dialect

import (
	"bufio"
	"bytes"
	"fmt"
	"net/textproto"
	"sort"
	"strconv"
	"strings"
)

type HTTPMessage struct {
	Type       string
	StartLine  string
	Method     string
	Target     string
	Version    string
	StatusCode int
	Reason     string
	Headers    map[string][]string
	Body       string
}

func ParseHTTPMessage(src string) (HTTPMessage, error) {
	head, body := splitHTTPHeadBody(src)
	head = strings.ReplaceAll(strings.ReplaceAll(head, "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(head, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return HTTPMessage{}, &ParseError{Kind: "httpmsg", Message: "missing start line"}
	}

	msg, err := parseHTTPStartLine(strings.TrimRight(lines[0], " \t"))
	if err != nil {
		return HTTPMessage{}, err
	}
	headerText := strings.Join(lines[1:], "\r\n")
	headers, err := parseHTTPHeaders(headerText)
	if err != nil {
		return HTTPMessage{}, &ParseError{Kind: "httpmsg", Message: err.Error()}
	}
	msg.Headers = headers
	msg.Body = body
	return msg, nil
}

func EncodeHTTPMessage(msg HTTPMessage) (string, error) {
	startLine, err := formatHTTPStartLine(msg)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	buf.WriteString(startLine)
	buf.WriteString("\r\n")

	keySet := make(map[string]struct{}, len(msg.Headers))
	for key := range msg.Headers {
		if !IsHTTPHeaderFieldName(key) {
			return "", &ParseError{Kind: "httpmsg", Message: fmt.Sprintf("invalid header name %q", key)}
		}
		keySet[textproto.CanonicalMIMEHeaderKey(key)] = struct{}{}
	}
	keys := make([]string, 0, len(keySet))
	for key := range keySet {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	canonical := make(map[string][]string, len(msg.Headers))
	for key, values := range msg.Headers {
		name := textproto.CanonicalMIMEHeaderKey(key)
		canonical[name] = append(canonical[name], values...)
	}
	for _, key := range keys {
		for _, val := range canonical[key] {
			if strings.ContainsAny(val, "\r\n") {
				return "", &ParseError{Kind: "httpmsg", Message: fmt.Sprintf("invalid header value for %q", key)}
			}
			buf.WriteString(key)
			buf.WriteString(": ")
			buf.WriteString(val)
			buf.WriteString("\r\n")
		}
	}
	buf.WriteString("\r\n")
	buf.WriteString(msg.Body)
	return buf.String(), nil
}

func IsHTTPHeaderFieldName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
			continue
		}
		switch c {
		case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
			continue
		default:
			return false
		}
	}
	return true
}

func splitHTTPHeadBody(src string) (string, string) {
	if idx := strings.Index(src, "\r\n\r\n"); idx >= 0 {
		return src[:idx], src[idx+4:]
	}
	if idx := strings.Index(src, "\n\n"); idx >= 0 {
		return src[:idx], src[idx+2:]
	}
	if idx := strings.Index(src, "\r\r"); idx >= 0 {
		return src[:idx], src[idx+2:]
	}
	return src, ""
}

func parseHTTPStartLine(line string) (HTTPMessage, error) {
	if strings.HasPrefix(line, "HTTP/") {
		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 2 {
			return HTTPMessage{}, &ParseError{Kind: "httpmsg", Message: "invalid response start line"}
		}
		status, err := strconv.Atoi(parts[1])
		if err != nil || status < 100 || status > 999 {
			return HTTPMessage{}, &ParseError{Kind: "httpmsg", Message: "invalid response status code"}
		}
		reason := ""
		if len(parts) == 3 {
			reason = parts[2]
		}
		return HTTPMessage{Type: "response", StartLine: line, Version: parts[0], StatusCode: status, Reason: reason}, nil
	}

	parts := strings.Split(line, " ")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || !strings.HasPrefix(parts[2], "HTTP/") {
		return HTTPMessage{}, &ParseError{Kind: "httpmsg", Message: "invalid request start line"}
	}
	return HTTPMessage{Type: "request", StartLine: line, Method: parts[0], Target: parts[1], Version: parts[2]}, nil
}

func parseHTTPHeaders(src string) (map[string][]string, error) {
	reader := textproto.NewReader(bufio.NewReader(strings.NewReader(src + "\r\n\r\n")))
	mimeHeader, err := reader.ReadMIMEHeader()
	if err != nil {
		return nil, err
	}
	out := make(map[string][]string, len(mimeHeader))
	for key, vals := range mimeHeader {
		copied := make([]string, len(vals))
		copy(copied, vals)
		out[key] = copied
	}
	return out, nil
}

func formatHTTPStartLine(msg HTTPMessage) (string, error) {
	if msg.StartLine != "" {
		parsed, err := parseHTTPStartLine(msg.StartLine)
		if err != nil {
			return "", err
		}
		if msg.Type != "" && parsed.Type != msg.Type {
			return "", &ParseError{Kind: "httpmsg", Message: "start line type does not match message type"}
		}
		return msg.StartLine, nil
	}

	version := msg.Version
	if version == "" {
		version = "HTTP/1.1"
	}
	if !strings.HasPrefix(version, "HTTP/") || strings.ContainsAny(version, " \t\r\n") {
		return "", &ParseError{Kind: "httpmsg", Message: "invalid HTTP version"}
	}

	switch msg.Type {
	case "", "request":
		if msg.Method == "" {
			return "", &ParseError{Kind: "httpmsg", Message: "request method required"}
		}
		target := msg.Target
		if target == "" {
			target = "/"
		}
		if strings.ContainsAny(msg.Method, " \t\r\n") || strings.ContainsAny(target, " \t\r\n") {
			return "", &ParseError{Kind: "httpmsg", Message: "invalid request start line"}
		}
		return msg.Method + " " + target + " " + version, nil
	case "response":
		if msg.StatusCode < 100 || msg.StatusCode > 999 {
			return "", &ParseError{Kind: "httpmsg", Message: "response status code required"}
		}
		if strings.ContainsAny(msg.Reason, "\r\n") {
			return "", &ParseError{Kind: "httpmsg", Message: "invalid response reason"}
		}
		line := version + " " + strconv.Itoa(msg.StatusCode)
		if msg.Reason != "" {
			line += " " + msg.Reason
		}
		return line, nil
	default:
		return "", &ParseError{Kind: "httpmsg", Message: fmt.Sprintf("unknown message type %q", msg.Type)}
	}
}
