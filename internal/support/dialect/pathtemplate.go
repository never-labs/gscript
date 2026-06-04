package dialect

import (
	"fmt"
	"net/url"
	"strings"
)

type PathTemplateMatch struct {
	Template string
	Path     string
	Params   map[string]string
	Matched  bool
}

type pathTemplateSegment struct {
	literal  string
	name     string
	catchAll bool
}

func MatchPathTemplate(template, path string) (PathTemplateMatch, error) {
	segments, err := parsePathTemplate(template)
	if err != nil {
		return PathTemplateMatch{}, err
	}
	pathSegments, err := splitPathSegments(path, false)
	if err != nil {
		return PathTemplateMatch{}, err
	}

	params := make(map[string]string)
	for i, segment := range segments {
		if segment.catchAll {
			rest := pathSegments[i:]
			parts := make([]string, 0, len(rest))
			for _, val := range rest {
				parts = append(parts, val.decoded)
			}
			params[segment.name] = strings.Join(parts, "/")
			return PathTemplateMatch{Template: template, Path: path, Params: params, Matched: true}, nil
		}
		if i >= len(pathSegments) {
			return PathTemplateMatch{Template: template, Path: path, Params: map[string]string{}, Matched: false}, nil
		}
		if segment.name != "" {
			params[segment.name] = pathSegments[i].decoded
			continue
		}
		if segment.literal != pathSegments[i].decoded {
			return PathTemplateMatch{Template: template, Path: path, Params: map[string]string{}, Matched: false}, nil
		}
	}
	if len(pathSegments) != len(segments) {
		return PathTemplateMatch{Template: template, Path: path, Params: map[string]string{}, Matched: false}, nil
	}
	return PathTemplateMatch{Template: template, Path: path, Params: params, Matched: true}, nil
}

func ExpandPathTemplate(template string, params map[string]string) (string, error) {
	segments, err := parsePathTemplate(template)
	if err != nil {
		return "", err
	}
	out := make([]string, 0, len(segments))
	for _, segment := range segments {
		switch {
		case segment.catchAll:
			val, ok := params[segment.name]
			if !ok {
				return "", &ParseError{Kind: "pathtemplate", Message: fmt.Sprintf("missing value for %q", segment.name)}
			}
			if val == "" {
				continue
			}
			for _, part := range strings.Split(val, "/") {
				out = append(out, url.PathEscape(part))
			}
		case segment.name != "":
			val, ok := params[segment.name]
			if !ok {
				return "", &ParseError{Kind: "pathtemplate", Message: fmt.Sprintf("missing value for %q", segment.name)}
			}
			if strings.Contains(val, "/") {
				return "", &ParseError{Kind: "pathtemplate", Message: fmt.Sprintf("value for %q contains slash", segment.name)}
			}
			out = append(out, url.PathEscape(val))
		default:
			out = append(out, segment.literal)
		}
	}
	return "/" + strings.Join(out, "/"), nil
}

type pathSegment struct {
	decoded string
}

func parsePathTemplate(template string) ([]pathTemplateSegment, error) {
	if template == "" || template[0] != '/' {
		return nil, &ParseError{Kind: "pathtemplate", Message: "template must start with /"}
	}
	rawSegments, err := splitPathSegments(template, true)
	if err != nil {
		return nil, err
	}
	segments := make([]pathTemplateSegment, 0, len(rawSegments))
	seen := make(map[string]struct{})
	for i, raw := range rawSegments {
		text := raw.decoded
		if strings.HasPrefix(text, "{") || strings.HasSuffix(text, "}") {
			if !strings.HasPrefix(text, "{") || !strings.HasSuffix(text, "}") || len(text) < 3 {
				return nil, &ParseError{Kind: "pathtemplate", Message: fmt.Sprintf("invalid placeholder %q", text)}
			}
			name := strings.TrimSuffix(strings.TrimPrefix(text, "{"), "}")
			catchAll := strings.HasPrefix(name, "*")
			if catchAll {
				name = strings.TrimPrefix(name, "*")
				if i != len(rawSegments)-1 {
					return nil, &ParseError{Kind: "pathtemplate", Message: "catch-all placeholder must be last"}
				}
			}
			if !isPathTemplateName(name) {
				return nil, &ParseError{Kind: "pathtemplate", Message: fmt.Sprintf("invalid placeholder name %q", name)}
			}
			if _, ok := seen[name]; ok {
				return nil, &ParseError{Kind: "pathtemplate", Message: fmt.Sprintf("duplicate placeholder %q", name)}
			}
			seen[name] = struct{}{}
			segments = append(segments, pathTemplateSegment{name: name, catchAll: catchAll})
			continue
		}
		segments = append(segments, pathTemplateSegment{literal: text})
	}
	return segments, nil
}

func splitPathSegments(path string, template bool) ([]pathSegment, error) {
	if path == "" || path[0] != '/' {
		kind := "path"
		if template {
			kind = "template"
		}
		return nil, &ParseError{Kind: "pathtemplate", Message: kind + " must start with /"}
	}
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return nil, nil
	}
	raw := strings.Split(trimmed, "/")
	out := make([]pathSegment, 0, len(raw))
	for _, part := range raw {
		decoded, err := url.PathUnescape(part)
		if err != nil {
			return nil, &ParseError{Kind: "pathtemplate", Message: err.Error()}
		}
		out = append(out, pathSegment{decoded: decoded})
	}
	return out, nil
}

func isPathTemplateName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c == '_' {
			continue
		}
		if i > 0 && c >= '0' && c <= '9' {
			continue
		}
		return false
	}
	return true
}
