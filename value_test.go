package gscript_test

import (
	"reflect"
	"testing"

	gs "github.com/never-labs/gscript"
)

func TestPublicValueConstructorsAndAccessors(t *testing.T) {
	tests := []struct {
		name string
		val  gs.Value
		kind gs.Kind
		text string
	}{
		{name: "nil", val: gs.Nil(), kind: gs.KindNil, text: "nil"},
		{name: "bool", val: gs.Bool(true), kind: gs.KindBool, text: "true"},
		{name: "int", val: gs.Int(42), kind: gs.KindInt, text: "42"},
		{name: "float", val: gs.Float(1.5), kind: gs.KindFloat, text: "1.5"},
		{name: "string", val: gs.String("hello"), kind: gs.KindString, text: "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.val.Kind(); got != tt.kind {
				t.Fatalf("Kind() = %q, want %q", got, tt.kind)
			}
			if got := tt.val.String(); got != tt.text {
				t.Fatalf("String() = %q, want %q", got, tt.text)
			}
		})
	}

	if got := gs.Bool(true).Bool(); !got {
		t.Fatalf("Bool() = false, want true")
	}
	if got := gs.Int(42).Int(); got != 42 {
		t.Fatalf("Int() = %d, want 42", got)
	}
	if got := gs.Float(1.5).Float(); got != 1.5 {
		t.Fatalf("Float() = %v, want 1.5", got)
	}
	if got := gs.Int(42).Float(); got != 42 {
		t.Fatalf("Int().Float() = %v, want 42", got)
	}
}

func TestPublicValueZeroValueIsNil(t *testing.T) {
	var v gs.Value
	if got := v.Kind(); got != gs.KindNil {
		t.Fatalf("zero Kind() = %q, want %q", got, gs.KindNil)
	}
	if !v.IsNil() {
		t.Fatal("zero Value should be nil")
	}
}

func TestPublicValueWorksWithInterfaceConversion(t *testing.T) {
	public := gs.String("bridge")
	decoded, err := gs.Decode(public)
	if err != nil {
		t.Fatal(err)
	}
	if got := decoded.String(); got != "bridge" {
		t.Fatalf("Decode(public).String() = %q, want bridge", got)
	}
}

func TestPublicValueEncodeDecode(t *testing.T) {
	v, err := gs.Decode(map[string]int{"answer": 42})
	if err != nil {
		t.Fatal(err)
	}
	if got := v.Kind(); got != gs.KindTable {
		t.Fatalf("Kind() = %q, want table", got)
	}

	encoded, err := v.Encode()
	if err != nil {
		t.Fatal(err)
	}
	m, ok := encoded.(map[string]interface{})
	if !ok {
		t.Fatalf("Encode() type = %T, want map[string]interface{}", encoded)
	}
	if got := m["answer"]; got != int64(42) {
		t.Fatalf("encoded answer = %v (%T), want int64(42)", got, got)
	}
}

func TestPublicValueDecodeHelpers(t *testing.T) {
	v, err := gs.Decode("hello")
	if err != nil {
		t.Fatal(err)
	}
	if got := v.String(); got != "hello" {
		t.Fatalf("Decode().String() = %q, want hello", got)
	}

	decoded := gs.MustDecode(true)
	if !decoded.Bool() {
		t.Fatalf("MustDecode(true).Bool() = false, want true")
	}
}

func TestPublicValueDecodeMethodAndTypedConversion(t *testing.T) {
	var v gs.Value
	if err := v.Decode("123"); err != nil {
		t.Fatal(err)
	}
	if got := v.Kind(); got != gs.KindString {
		t.Fatalf("Kind() = %q, want string", got)
	}

	rv, err := v.To(reflect.TypeOf(int64(0)))
	if err != nil {
		t.Fatal(err)
	}
	if got := rv.Interface(); got != int64(123) {
		t.Fatalf("To(int64) = %v (%T), want int64(123)", got, got)
	}
}
