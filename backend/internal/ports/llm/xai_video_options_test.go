package llm

import (
	"math"
	"testing"
)

func TestIntegerOption(t *testing.T) {
	tests := []struct {
		name   string
		value  any
		want   int
		wantOK bool
	}{
		{name: "int", value: 3, want: 3, wantOK: true},
		{name: "int64", value: int64(4), want: 4, wantOK: true},
		{name: "integral float", value: float64(5), want: 5, wantOK: true},
		{name: "fractional float", value: 1.5},
		{name: "overflow", value: math.Inf(1)},
		{name: "string", value: "6"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := IntegerOption(map[string]any{"value": test.value}, "value")
			if got != test.want || ok != test.wantOK {
				t.Fatalf("IntegerOption() = (%d, %v), want (%d, %v)", got, ok, test.want, test.wantOK)
			}
		})
	}
}
