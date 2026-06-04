package dialect

import (
	"fmt"
	"io"
	"mime"
	stdmultipart "mime/multipart"
	"net/textproto"
	"sort"
	"strings"
)

type MultipartPart struct {
	Name        string
	Filename    string
	ContentType string
	Body        string
	Headers     map[string][]string
}

func ParseMultipart(src, boundary string) ([]MultipartPart, error) {
	if err := validateMultipartBoundary(boundary); err != nil {
		return nil, err
	}
	reader := stdmultipart.NewReader(strings.NewReader(src), boundary)
	var parts []MultipartPart
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(part)
		if err != nil {
			return nil, err
		}
		headers := cloneMIMEHeader(part.Header)
		parts = append(parts, MultipartPart{
			Name:        part.FormName(),
			Filename:    part.FileName(),
			ContentType: firstHeader(headers, "Content-Type"),
			Body:        string(data),
			Headers:     headers,
		})
	}
	return parts, nil
}

func EncodeMultipart(parts []MultipartPart, boundary string) (string, error) {
	if err := validateMultipartBoundary(boundary); err != nil {
		return "", err
	}
	var b strings.Builder
	for _, part := range parts {
		b.WriteString("--")
		b.WriteString(boundary)
		b.WriteString("\r\n")
		headers := multipartPartHeaders(part)
		keys := make([]string, 0, len(headers))
		for key := range headers {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			values := append([]string(nil), headers[key]...)
			for _, value := range values {
				if strings.ContainsAny(value, "\r\n") {
					return "", fmt.Errorf("multipart: invalid header value for %q", key)
				}
				b.WriteString(key)
				b.WriteString(": ")
				b.WriteString(value)
				b.WriteString("\r\n")
			}
		}
		b.WriteString("\r\n")
		b.WriteString(part.Body)
		b.WriteString("\r\n")
	}
	b.WriteString("--")
	b.WriteString(boundary)
	b.WriteString("--\r\n")
	return b.String(), nil
}

func validateMultipartBoundary(boundary string) error {
	if boundary == "" {
		return fmt.Errorf("multipart: boundary required")
	}
	writer := stdmultipart.NewWriter(io.Discard)
	if err := writer.SetBoundary(boundary); err != nil {
		return fmt.Errorf("multipart: invalid boundary %q: %w", boundary, err)
	}
	return nil
}

func multipartPartHeaders(part MultipartPart) textproto.MIMEHeader {
	headers := textproto.MIMEHeader{}
	for key, values := range part.Headers {
		name := textproto.CanonicalMIMEHeaderKey(key)
		if name == "" {
			continue
		}
		for _, value := range values {
			headers.Add(name, value)
		}
	}
	if part.Name != "" && firstHeader(headers, "Content-Disposition") == "" {
		params := map[string]string{"name": part.Name}
		if part.Filename != "" {
			params["filename"] = part.Filename
		}
		headers.Set("Content-Disposition", mime.FormatMediaType("form-data", params))
	}
	if part.ContentType != "" && firstHeader(headers, "Content-Type") == "" {
		headers.Set("Content-Type", part.ContentType)
	}
	return headers
}

func cloneMIMEHeader(header textproto.MIMEHeader) map[string][]string {
	out := make(map[string][]string, len(header))
	for key, values := range header {
		out[textproto.CanonicalMIMEHeaderKey(key)] = append([]string(nil), values...)
	}
	return out
}

func firstHeader(headers map[string][]string, key string) string {
	values := headers[textproto.CanonicalMIMEHeaderKey(key)]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
