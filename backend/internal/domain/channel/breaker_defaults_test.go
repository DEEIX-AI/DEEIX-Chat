package channel

import "testing"

func TestBreakerDefaultsValid(t *testing.T) {
	maxMinutes := int(^uint(0)>>1) / 60
	tests := []struct {
		name     string
		defaults BreakerDefaults
		want     bool
	}{
		{name: "empty uses built-in defaults", defaults: BreakerDefaults{}, want: true},
		{name: "supported logic", defaults: BreakerDefaults{ModelFailureThreshold: 5, UpstreamThresholdLogic: "and"}, want: true},
		{name: "negative threshold", defaults: BreakerDefaults{ModelFailureThreshold: -1}, want: false},
		{name: "unsupported logic", defaults: BreakerDefaults{UpstreamThresholdLogic: "xor"}, want: false},
		{name: "minute conversion overflow", defaults: BreakerDefaults{ModelDurationMin: maxMinutes + 1}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.defaults.Valid(); got != tt.want {
				t.Fatalf("BreakerDefaults.Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}
