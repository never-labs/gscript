package binfmt

import (
	"encoding/binary"
	"testing"
)

func TestParseFormatOrderAndFields(t *testing.T) {
	format, err := Parse("be:u16, string:4 <f32 bytes")
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if format.Order != binary.LittleEndian {
		t.Fatalf("Order = %T, want little endian", format.Order)
	}
	want := []Field{
		{Kind: "u16", Count: -1},
		{Kind: "string", Count: 4},
		{Kind: "f32", Count: -1},
		{Kind: "bytes", Count: -1},
	}
	if len(format.Fields) != len(want) {
		t.Fatalf("len(Fields) = %d, want %d", len(format.Fields), len(want))
	}
	for i := range want {
		if format.Fields[i] != want[i] {
			t.Fatalf("Fields[%d] = %#v, want %#v", i, format.Fields[i], want[i])
		}
	}
}

func TestParseFormatErrors(t *testing.T) {
	if _, err := Parse("bytes:-1"); err == nil {
		t.Fatal("Parse(bytes:-1) returned nil error")
	}
	if _, err := Parse("bogus"); err == nil {
		t.Fatal("Parse(bogus) returned nil error")
	}
}

func TestFieldSize(t *testing.T) {
	tests := []struct {
		name  string
		field Field
		size  int
		fixed bool
	}{
		{name: "u8", field: Field{Kind: "u8", Count: -1}, size: 1, fixed: true},
		{name: "f64", field: Field{Kind: "f64", Count: -1}, size: 8, fixed: true},
		{name: "fixed string", field: Field{Kind: "string", Count: 3}, size: 3, fixed: true},
		{name: "variable bytes", field: Field{Kind: "bytes", Count: -1}, size: 0, fixed: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size, fixed := FieldSize(tt.field)
			if size != tt.size || fixed != tt.fixed {
				t.Fatalf("FieldSize(%#v) = %d, %t; want %d, %t", tt.field, size, fixed, tt.size, tt.fixed)
			}
		})
	}
}
