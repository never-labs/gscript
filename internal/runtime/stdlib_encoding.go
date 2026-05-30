package runtime

import (
	"encoding/base32"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// buildEncodingLib creates the "encoding" standard library table.
// Provides hex, base32, ini, and xml encoding/decoding utilities.
// Inspired by Odin's encoding package family.
func buildEncodingLib(interps ...*Interpreter) *Table {
	t := NewTable()
	var interp *Interpreter
	if len(interps) > 0 {
		interp = interps[0]
	}
	maxHostResult := func() int64 {
		if interp == nil {
			return 0
		}
		return interp.maxHostResult
	}

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name: "encoding." + name,
			Fn:   fn,
		}))
	}

	// ---------------------------------------------------------------
	// Hex encoding
	// ---------------------------------------------------------------

	// encoding.hexEncode(str) -> hex-encoded string
	set("hexEncode", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'encoding.hexEncode' (string expected)")
		}
		if err := CheckProjectedHostStringBytes(maxHostResult(), hex.EncodedLen(StringLen(args[0]))); err != nil {
			return nil, err
		}
		encoded := hex.EncodeToString([]byte(args[0].Str()))
		return []Value{StringValue(encoded)}, nil
	})

	// encoding.hexDecode(hexStr) -> decoded string, or nil, error
	set("hexDecode", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'encoding.hexDecode' (string expected)")
		}
		if err := CheckProjectedHostStringBytes(maxHostResult(), hex.DecodedLen(StringLen(args[0]))); err != nil {
			return nil, err
		}
		decoded, err := hex.DecodeString(args[0].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(string(decoded))}, nil
	})

	// ---------------------------------------------------------------
	// Base32 encoding
	// ---------------------------------------------------------------

	// encoding.base32Encode(str) -> base32-encoded string
	set("base32Encode", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'encoding.base32Encode' (string expected)")
		}
		if err := CheckProjectedHostStringBytes(maxHostResult(), base32.StdEncoding.EncodedLen(StringLen(args[0]))); err != nil {
			return nil, err
		}
		encoded := base32.StdEncoding.EncodeToString([]byte(args[0].Str()))
		return []Value{StringValue(encoded)}, nil
	})

	// encoding.base32Decode(str) -> decoded string, or nil, error
	set("base32Decode", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'encoding.base32Decode' (string expected)")
		}
		if err := CheckProjectedHostStringBytes(maxHostResult(), base32.StdEncoding.DecodedLen(StringLen(args[0]))); err != nil {
			return nil, err
		}
		decoded, err := base32.StdEncoding.DecodeString(args[0].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(string(decoded))}, nil
	})

	// encoding.base32HexEncode(str) -> base32hex-encoded string (no padding)
	set("base32HexEncode", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'encoding.base32HexEncode' (string expected)")
		}
		enc := base32.HexEncoding.WithPadding(base32.NoPadding)
		if err := CheckProjectedHostStringBytes(maxHostResult(), enc.EncodedLen(StringLen(args[0]))); err != nil {
			return nil, err
		}
		encoded := enc.EncodeToString([]byte(args[0].Str()))
		return []Value{StringValue(encoded)}, nil
	})

	// encoding.base32HexDecode(str) -> decoded string, or nil, error
	set("base32HexDecode", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'encoding.base32HexDecode' (string expected)")
		}
		enc := base32.HexEncoding.WithPadding(base32.NoPadding)
		if err := CheckProjectedHostStringBytes(maxHostResult(), enc.DecodedLen(StringLen(args[0]))); err != nil {
			return nil, err
		}
		decoded, err := enc.DecodeString(args[0].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(string(decoded))}, nil
	})

	// ---------------------------------------------------------------
	// INI encoding
	// ---------------------------------------------------------------

	// encoding.iniEncode(table) -> INI-formatted string
	// Table can be flat (key=value) or nested (section.key=value)
	set("iniEncode", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'encoding.iniEncode' (table expected)")
		}
		tbl := args[0].Table()
		var sb strings.Builder

		// Collect global keys and section keys
		var globalKeys []string
		sections := make(map[string]*Table)
		var sectionOrder []string

		key := NilValue()
		for {
			k, v, ok := tbl.Next(key)
			if !ok {
				break
			}
			if k.IsString() {
				if v.IsTable() {
					sections[k.Str()] = v.Table()
					sectionOrder = append(sectionOrder, k.Str())
				} else {
					globalKeys = append(globalKeys, k.Str())
				}
			}
			key = k
		}

		// Write global keys first
		for _, gk := range globalKeys {
			v := tbl.RawGet(StringValue(gk))
			sb.WriteString(fmt.Sprintf("%s=%s\n", gk, v.String()))
		}

		// Write sections
		for _, secName := range sectionOrder {
			secTbl := sections[secName]
			if len(globalKeys) > 0 || secName != sectionOrder[0] {
				sb.WriteString("\n")
			}
			sb.WriteString(fmt.Sprintf("[%s]\n", secName))
			sKey := NilValue()
			for {
				sk, sv, ok := secTbl.Next(sKey)
				if !ok {
					break
				}
				if sk.IsString() {
					sb.WriteString(fmt.Sprintf("%s=%s\n", sk.Str(), sv.String()))
				}
				sKey = sk
			}
		}

		return []Value{StringValue(sb.String())}, nil
	})

	// encoding.iniDecode(str) -> table
	// Parses INI-formatted string into a table with sections as nested tables
	set("iniDecode", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'encoding.iniDecode' (string expected)")
		}
		content := args[0].Str()
		result := NewTable()
		var currentSection *Table

		lines := strings.Split(content, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			// Skip empty lines and comments
			if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
				continue
			}
			// Section header
			if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
				secName := strings.TrimSpace(line[1 : len(line)-1])
				currentSection = NewTable()
				result.RawSet(StringValue(secName), TableValue(currentSection))
				continue
			}
			// Key=Value pair
			eqIdx := strings.IndexByte(line, '=')
			if eqIdx < 0 {
				continue
			}
			key := strings.TrimSpace(line[:eqIdx])
			val := strings.TrimSpace(line[eqIdx+1:])
			if currentSection != nil {
				currentSection.RawSet(StringValue(key), StringValue(val))
			} else {
				result.RawSet(StringValue(key), StringValue(val))
			}
		}

		return []Value{TableValue(result)}, nil
	})

	// ---------------------------------------------------------------
	// XML encoding (simplified)
	// ---------------------------------------------------------------

	// encoding.xmlEscape(str) -> XML-escaped string
	set("xmlEscape", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'encoding.xmlEscape' (string expected)")
		}
		var sb strings.Builder
		xml.EscapeText(&sb, []byte(args[0].Str()))
		return []Value{StringValue(sb.String())}, nil
	})

	// encoding.xmlUnescape(str) -> unescaped string, or nil, error
	set("xmlUnescape", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'encoding.xmlUnescape' (string expected)")
		}
		s := args[0].Str()
		var err error
		s, err = xmlUnescapeNumericRefs(s)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		// Simple XML unescape for common entities. Keep &amp; last so an escaped
		// entity name such as &amp;lt; is decoded only once.
		s = strings.ReplaceAll(s, "&lt;", "<")
		s = strings.ReplaceAll(s, "&gt;", ">")
		s = strings.ReplaceAll(s, "&quot;", "\"")
		s = strings.ReplaceAll(s, "&apos;", "'")
		s = strings.ReplaceAll(s, "&amp;", "&")
		return []Value{StringValue(s)}, nil
	})

	return t
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
