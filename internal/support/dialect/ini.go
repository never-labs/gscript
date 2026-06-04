package dialect

import (
	"fmt"
	"sort"
	"strings"
)

type INIDocument struct {
	Root     map[string]string
	Sections map[string]map[string]string
}

func ParseINI(src string) (INIDocument, error) {
	doc := INIDocument{
		Root:     make(map[string]string),
		Sections: make(map[string]map[string]string),
	}
	current := doc.Root
	for lineNo, line := range Lines(src, true, false) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			if !strings.HasSuffix(line, "]") {
				return INIDocument{}, &ParseError{Kind: "ini", Message: fmt.Sprintf("line %d: malformed section header", lineNo+1)}
			}
			name := strings.TrimSpace(line[1 : len(line)-1])
			if name == "" {
				return INIDocument{}, &ParseError{Kind: "ini", Message: fmt.Sprintf("line %d: empty section name", lineNo+1)}
			}
			section := doc.Sections[name]
			if section == nil {
				section = make(map[string]string)
				doc.Sections[name] = section
			}
			current = section
			continue
		}
		key, val, ok := cutINIKeyValue(line)
		if !ok {
			return INIDocument{}, &ParseError{Kind: "ini", Message: fmt.Sprintf("line %d: missing key/value separator", lineNo+1)}
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return INIDocument{}, &ParseError{Kind: "ini", Message: fmt.Sprintf("line %d: empty key", lineNo+1)}
		}
		current[key] = strings.TrimSpace(val)
	}
	return doc, nil
}

func EncodeINI(doc INIDocument) (string, error) {
	var b strings.Builder
	if err := writeINIFields(&b, doc.Root); err != nil {
		return "", err
	}
	sections := sortedMapKeys(doc.Sections)
	for _, section := range sections {
		if section == "" || strings.ContainsAny(section, "\r\n[]") {
			return "", &ParseError{Kind: "ini", Message: "invalid section name " + fmt.Sprintf("%q", section)}
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteByte('[')
		b.WriteString(section)
		b.WriteString("]\n")
		if err := writeINIFields(&b, doc.Sections[section]); err != nil {
			return "", err
		}
	}
	return b.String(), nil
}

func cutINIKeyValue(line string) (string, string, bool) {
	eq := strings.IndexByte(line, '=')
	colon := strings.IndexByte(line, ':')
	switch {
	case eq < 0 && colon < 0:
		return "", "", false
	case eq < 0:
		return line[:colon], line[colon+1:], true
	case colon < 0:
		return line[:eq], line[eq+1:], true
	case eq < colon:
		return line[:eq], line[eq+1:], true
	default:
		return line[:colon], line[colon+1:], true
	}
}

func writeINIFields(b *strings.Builder, fields map[string]string) error {
	for _, key := range sortedMapKeys(fields) {
		if key == "" || strings.ContainsAny(key, "\r\n=:[]") {
			return &ParseError{Kind: "ini", Message: "invalid key " + fmt.Sprintf("%q", key)}
		}
		value := fields[key]
		if strings.ContainsAny(value, "\r\n") {
			return &ParseError{Kind: "ini", Message: "invalid multiline value for key " + fmt.Sprintf("%q", key)}
		}
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(value)
		b.WriteByte('\n')
	}
	return nil
}

func sortedMapKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
