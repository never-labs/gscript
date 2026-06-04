package dialect

import (
	"fmt"
	"html"
	"net/url"
	"sort"
	"strings"
)

type KVOptions struct {
	Sep     string
	Trim    bool
	EnvMode bool
}

type URLParts struct {
	Scheme   string
	Host     string
	Port     string
	Path     string
	Fragment string
	Raw      string
	User     string
	Password *string
	HasUser  bool
	Query    map[string]string
}

func Lines(src string, keepEmpty, keepTrailing bool) []string {
	normalized := strings.ReplaceAll(strings.ReplaceAll(src, "\r\n", "\n"), "\r", "\n")
	parts := strings.Split(normalized, "\n")
	if !keepTrailing && len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	if keepEmpty {
		return parts
	}
	out := parts[:0]
	for _, line := range parts {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func Words(src string) []string {
	return strings.Fields(src)
}

func KV(src string, opts KVOptions) (map[string]string, error) {
	sep := opts.Sep
	if sep == "" {
		sep = "="
	}
	out := make(map[string]string)
	for _, line := range Lines(src, true, false) {
		if opts.Trim {
			line = strings.TrimSpace(line)
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, sep)
		if !ok && opts.EnvMode {
			key, val, ok = strings.Cut(line, "=")
		}
		if !ok {
			return nil, &ParseError{Kind: "kv", Message: "missing separator in line " + line}
		}
		if opts.Trim {
			key = strings.TrimSpace(key)
			val = strings.TrimSpace(val)
		}
		if opts.EnvMode {
			val = strings.Trim(strings.TrimSpace(val), `"'`)
		}
		out[key] = val
	}
	return out, nil
}

func EncodeKV(values map[string]string, opts KVOptions) (string, error) {
	sep := opts.Sep
	if sep == "" {
		sep = "="
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		if key == "" {
			return "", fmt.Errorf("kv: empty key")
		}
		if strings.ContainsAny(key, "\r\n") || strings.Contains(key, sep) {
			return "", fmt.Errorf("kv: invalid key %q", key)
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, key := range keys {
		value := values[key]
		if strings.ContainsAny(value, "\r\n") {
			return "", fmt.Errorf("kv: invalid value for %q", key)
		}
		b.WriteString(key)
		b.WriteString(sep)
		b.WriteString(value)
		b.WriteByte('\n')
	}
	return b.String(), nil
}

func HTMLEscape(src string) string {
	return html.EscapeString(src)
}

func HTMLUnescape(src string) string {
	return html.UnescapeString(src)
}

func URLQueryEncode(values map[string]string) string {
	query := url.Values{}
	for key, val := range values {
		query.Set(key, val)
	}
	return query.Encode()
}

func URLQueryParse(src string) (map[string][]string, error) {
	return url.ParseQuery(src)
}

func URLQueryEscape(src string) string {
	return url.QueryEscape(src)
}

func URLQueryUnescape(src string) (string, error) {
	return url.QueryUnescape(src)
}

func URLPathEscape(src string) string {
	return url.PathEscape(src)
}

func URLPathUnescape(src string) (string, error) {
	return url.PathUnescape(src)
}

func ParseURL(src string) (URLParts, error) {
	u, err := url.Parse(src)
	if err != nil {
		return URLParts{}, err
	}
	query := make(map[string]string)
	for key, val := range u.Query() {
		query[key] = strings.Join(val, ",")
	}
	return URLParts{
		Scheme:   u.Scheme,
		Host:     u.Hostname(),
		Port:     u.Port(),
		Path:     u.Path,
		Fragment: u.Fragment,
		Raw:      u.String(),
		User:     u.User.Username(),
		Password: passwordPtr(u),
		HasUser:  u.User != nil,
		Query:    query,
	}, nil
}

func TemplateMissingKeyOption(mode string) string {
	switch mode {
	case "default":
		return "missingkey=default"
	case "error":
		return "missingkey=error"
	default:
		return "missingkey=zero"
	}
}

type ParseError struct {
	Kind    string
	Message string
}

func (e *ParseError) Error() string {
	return e.Kind + " dialect: " + e.Message
}

func passwordPtr(u *url.URL) *string {
	if u.User == nil {
		return nil
	}
	password, ok := u.User.Password()
	if !ok {
		return nil
	}
	return &password
}
