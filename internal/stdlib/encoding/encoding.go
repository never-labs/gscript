package encoding

import (
	"encoding/base32"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

type INIKeyValue struct {
	Key   string
	Value string
}

type INISection struct {
	Name  string
	Items []INIKeyValue
}

type INIDocument struct {
	Globals  []INIKeyValue
	Sections []INISection
}

func HexEncode(s string) string {
	return hex.EncodeToString([]byte(s))
}

func HexDecode(s string) (string, error) {
	decoded, err := hex.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func HexEncodedLen(n int) int {
	return hex.EncodedLen(n)
}

func HexDecodedLen(n int) int {
	return hex.DecodedLen(n)
}

func Base32Encode(s string) string {
	return base32.StdEncoding.EncodeToString([]byte(s))
}

func Base32Decode(s string) (string, error) {
	decoded, err := base32.StdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func Base32EncodedLen(n int) int {
	return base32.StdEncoding.EncodedLen(n)
}

func Base32DecodedLen(n int) int {
	return base32.StdEncoding.DecodedLen(n)
}

func Base32HexEncode(s string) string {
	return base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(s))
}

func Base32HexDecode(s string) (string, error) {
	decoded, err := base32.HexEncoding.WithPadding(base32.NoPadding).DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func Base32HexEncodedLen(n int) int {
	return base32.HexEncoding.WithPadding(base32.NoPadding).EncodedLen(n)
}

func Base32HexDecodedLen(n int) int {
	return base32.HexEncoding.WithPadding(base32.NoPadding).DecodedLen(n)
}

func EncodeINI(doc INIDocument) string {
	var sb strings.Builder
	for _, item := range doc.Globals {
		sb.WriteString(fmt.Sprintf("%s=%s\n", item.Key, item.Value))
	}
	for i, section := range doc.Sections {
		if len(doc.Globals) > 0 || i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(fmt.Sprintf("[%s]\n", section.Name))
		for _, item := range section.Items {
			sb.WriteString(fmt.Sprintf("%s=%s\n", item.Key, item.Value))
		}
	}
	return sb.String()
}

func DecodeINI(content string) INIDocument {
	var doc INIDocument
	currentSection := -1

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			secName := strings.TrimSpace(line[1 : len(line)-1])
			doc.Sections = append(doc.Sections, INISection{Name: secName})
			currentSection = len(doc.Sections) - 1
			continue
		}
		eqIdx := strings.IndexByte(line, '=')
		if eqIdx < 0 {
			continue
		}
		item := INIKeyValue{
			Key:   strings.TrimSpace(line[:eqIdx]),
			Value: strings.TrimSpace(line[eqIdx+1:]),
		}
		if currentSection >= 0 {
			doc.Sections[currentSection].Items = append(doc.Sections[currentSection].Items, item)
		} else {
			doc.Globals = append(doc.Globals, item)
		}
	}
	return doc
}

func XMLEscape(s string) string {
	var sb strings.Builder
	xml.EscapeText(&sb, []byte(s))
	return sb.String()
}

func XMLUnescape(s string) (string, error) {
	s, err := xmlUnescapeNumericRefs(s)
	if err != nil {
		return "", err
	}
	// Keep &amp; last so an escaped entity name such as &amp;lt; is decoded once.
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&apos;", "'")
	s = strings.ReplaceAll(s, "&amp;", "&")
	return s, nil
}

func xmlUnescapeNumericRefs(s string) (string, error) {
	var sb strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '&' || i+2 >= len(s) || s[i+1] != '#' {
			sb.WriteByte(s[i])
			i++
			continue
		}

		semi := strings.IndexByte(s[i+2:], ';')
		if semi < 0 {
			sb.WriteByte(s[i])
			i++
			continue
		}
		end := i + 2 + semi
		body := s[i+2 : end]
		if body == "" {
			return "", fmt.Errorf("invalid XML numeric character reference")
		}

		base := 10
		if body[0] == 'x' || body[0] == 'X' {
			base = 16
			body = body[1:]
			if body == "" {
				return "", fmt.Errorf("invalid XML numeric character reference")
			}
		}

		n, err := strconv.ParseInt(body, base, 32)
		if err != nil || !utf8.ValidRune(rune(n)) {
			return "", fmt.Errorf("invalid XML numeric character reference")
		}
		sb.WriteRune(rune(n))
		i = end + 1
	}
	return sb.String(), nil
}
