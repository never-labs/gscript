package runtime

import (
	"math"
	"testing"
)

func TestStringSubValueIntegerBoundaries(t *testing.T) {
	s := StringValue("123456789")
	min := IntValue(math.MinInt64)
	max := IntValue(math.MaxInt64)

	tests := []struct {
		name string
		run  func() (Value, error)
		want string
	}{
		{
			name: "minimum start",
			run: func() (Value, error) {
				return stringSub3Value(s, min, IntValue(-4))
			},
			want: "123456",
		},
		{
			name: "minimum start maximum end",
			run: func() (Value, error) {
				return stringSub3Value(s, min, max)
			},
			want: "123456789",
		},
		{
			name: "maximum start",
			run: func() (Value, error) {
				return stringSub2Value(s, max)
			},
			want: "",
		},
		{
			name: "minimum start and end",
			run: func() (Value, error) {
				return stringSubValue([]Value{s, min, min})
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.run()
			if err != nil {
				t.Fatalf("string.sub: %v", err)
			}
			if !got.IsString() || got.Str() != tt.want {
				t.Fatalf("string.sub = %q, want %q", got.Str(), tt.want)
			}
		})
	}
}
