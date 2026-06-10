package runtime

import (
	"errors"
	"reflect"
	"testing"
)

func TestNewDenseArrayFromCopyConstructors(t *testing.T) {
	i64, err := NewDenseArrayI64FromCopy(3, func(dst []int64) error {
		copy(dst, []int64{10, 20, 30})
		return nil
	})
	if err != nil {
		t.Fatalf("NewDenseArrayI64FromCopy returned error: %v", err)
	}
	if got, ok := i64.I64(); !ok || !reflect.DeepEqual(got, []int64{10, 20, 30}) {
		t.Fatalf("NewDenseArrayI64FromCopy = %v, %v", got, ok)
	}

	f64, err := NewDenseArrayF64FromCopy(2, func(dst []float64) error {
		copy(dst, []float64{1.5, -2.25})
		return nil
	})
	if err != nil {
		t.Fatalf("NewDenseArrayF64FromCopy returned error: %v", err)
	}
	if got, ok := f64.F64(); !ok || !reflect.DeepEqual(got, []float64{1.5, -2.25}) {
		t.Fatalf("NewDenseArrayF64FromCopy = %v, %v", got, ok)
	}

	bools, err := NewDenseArrayBoolFromCopy(3, func(dst []bool) error {
		copy(dst, []bool{true, false, true})
		return nil
	})
	if err != nil {
		t.Fatalf("NewDenseArrayBoolFromCopy returned error: %v", err)
	}
	if got, ok := bools.Bool(); !ok || !reflect.DeepEqual(got, []bool{true, false, true}) {
		t.Fatalf("NewDenseArrayBoolFromCopy = %v, %v", got, ok)
	}

	strings, err := NewDenseArrayStringFromCopy(2, func(dst []string) error {
		copy(dst, []string{"AAPL", "MSFT"})
		return nil
	})
	if err != nil {
		t.Fatalf("NewDenseArrayStringFromCopy returned error: %v", err)
	}
	if got, ok := strings.StringValues(); !ok || !reflect.DeepEqual(got, []string{"AAPL", "MSFT"}) {
		t.Fatalf("NewDenseArrayStringFromCopy = %v, %v", got, ok)
	}
}

func TestNewDenseArrayFromCopyErrors(t *testing.T) {
	wantErr := errors.New("copy failed")
	if _, err := NewDenseArrayI64FromCopy(1, func([]int64) error { return wantErr }); !errors.Is(err, wantErr) {
		t.Fatalf("NewDenseArrayI64FromCopy error = %v, want %v", err, wantErr)
	}
	if _, err := NewDenseArrayI64FromCopy(-1, func([]int64) error { return nil }); err == nil {
		t.Fatal("NewDenseArrayI64FromCopy negative length returned nil error")
	}
	if _, err := NewDenseArrayI64FromCopy(1, nil); !errors.Is(err, ErrDenseArrayOperand) {
		t.Fatalf("NewDenseArrayI64FromCopy nil copier error = %v, want ErrDenseArrayOperand", err)
	}
}
