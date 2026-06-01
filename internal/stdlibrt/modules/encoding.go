package modules

import (
	"fmt"

	stdencoding "github.com/never-labs/leia/internal/stdlib/encoding"
)

// BuildEncoding creates the "encoding" standard library table.
// Provides hex, base32, ini, and xml encoding/decoding utilities.
// Inspired by Odin's encoding package family.
func BuildEncoding(maxHostResult func() int64) *Table {
	t := NewTable()

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
		if err := CheckProjectedHostStringBytes(hostResultLimit(maxHostResult), stdencoding.HexEncodedLen(StringLen(args[0]))); err != nil {
			return nil, err
		}
		return []Value{StringValue(stdencoding.HexEncode(args[0].Str()))}, nil
	})

	// encoding.hexDecode(hexStr) -> decoded string, or nil, error
	set("hexDecode", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'encoding.hexDecode' (string expected)")
		}
		if err := CheckProjectedHostStringBytes(hostResultLimit(maxHostResult), stdencoding.HexDecodedLen(StringLen(args[0]))); err != nil {
			return nil, err
		}
		decoded, err := stdencoding.HexDecode(args[0].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(decoded)}, nil
	})

	// ---------------------------------------------------------------
	// Base32 encoding
	// ---------------------------------------------------------------

	// encoding.base32Encode(str) -> base32-encoded string
	set("base32Encode", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'encoding.base32Encode' (string expected)")
		}
		if err := CheckProjectedHostStringBytes(hostResultLimit(maxHostResult), stdencoding.Base32EncodedLen(StringLen(args[0]))); err != nil {
			return nil, err
		}
		return []Value{StringValue(stdencoding.Base32Encode(args[0].Str()))}, nil
	})

	// encoding.base32Decode(str) -> decoded string, or nil, error
	set("base32Decode", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'encoding.base32Decode' (string expected)")
		}
		if err := CheckProjectedHostStringBytes(hostResultLimit(maxHostResult), stdencoding.Base32DecodedLen(StringLen(args[0]))); err != nil {
			return nil, err
		}
		decoded, err := stdencoding.Base32Decode(args[0].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(decoded)}, nil
	})

	// encoding.base32HexEncode(str) -> base32hex-encoded string (no padding)
	set("base32HexEncode", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'encoding.base32HexEncode' (string expected)")
		}
		if err := CheckProjectedHostStringBytes(hostResultLimit(maxHostResult), stdencoding.Base32HexEncodedLen(StringLen(args[0]))); err != nil {
			return nil, err
		}
		return []Value{StringValue(stdencoding.Base32HexEncode(args[0].Str()))}, nil
	})

	// encoding.base32HexDecode(str) -> decoded string, or nil, error
	set("base32HexDecode", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'encoding.base32HexDecode' (string expected)")
		}
		if err := CheckProjectedHostStringBytes(hostResultLimit(maxHostResult), stdencoding.Base32HexDecodedLen(StringLen(args[0]))); err != nil {
			return nil, err
		}
		decoded, err := stdencoding.Base32HexDecode(args[0].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(decoded)}, nil
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
		doc := stdencoding.INIDocument{}

		key := NilValue()
		for {
			k, v, ok := tbl.Next(key)
			if !ok {
				break
			}
			if k.IsString() {
				if v.IsTable() {
					section := stdencoding.INISection{Name: k.Str()}
					secTbl := v.Table()
					sKey := NilValue()
					for {
						sk, sv, ok := secTbl.Next(sKey)
						if !ok {
							break
						}
						if sk.IsString() {
							section.Items = append(section.Items, stdencoding.INIKeyValue{Key: sk.Str(), Value: sv.String()})
						}
						sKey = sk
					}
					doc.Sections = append(doc.Sections, section)
				} else {
					doc.Globals = append(doc.Globals, stdencoding.INIKeyValue{Key: k.Str(), Value: v.String()})
				}
			}
			key = k
		}

		return []Value{StringValue(stdencoding.EncodeINI(doc))}, nil
	})

	// encoding.iniDecode(str) -> table
	// Parses INI-formatted string into a table with sections as nested tables
	set("iniDecode", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'encoding.iniDecode' (string expected)")
		}
		result := NewTable()
		doc := stdencoding.DecodeINI(args[0].Str())
		for _, item := range doc.Globals {
			result.RawSet(StringValue(item.Key), StringValue(item.Value))
		}
		for _, section := range doc.Sections {
			secTbl := NewTable()
			for _, item := range section.Items {
				secTbl.RawSet(StringValue(item.Key), StringValue(item.Value))
			}
			result.RawSet(StringValue(section.Name), TableValue(secTbl))
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
		return []Value{StringValue(stdencoding.XMLEscape(args[0].Str()))}, nil
	})

	// encoding.xmlUnescape(str) -> unescaped string, or nil, error
	set("xmlUnescape", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'encoding.xmlUnescape' (string expected)")
		}
		s, err := stdencoding.XMLUnescape(args[0].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(s)}, nil
	})

	return t
}
