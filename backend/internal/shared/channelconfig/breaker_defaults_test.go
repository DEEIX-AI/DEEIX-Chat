package channelconfig

import (
	"errors"
	"reflect"
	"testing"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
)

func TestParseBreakerDefaults(t *testing.T) {
	t.Run("legacy config remains disabled and receives defaults", func(t *testing.T) {
		got, err := ParseBreakerDefaults(`{"model_failure_threshold":9}`)
		if err != nil {
			t.Fatalf("ParseBreakerDefaults() error = %v", err)
		}
		want := domainchannel.DefaultBreakerDefaults()
		want.ModelFailureThreshold = 9
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("ParseBreakerDefaults() = %#v, want %#v", got, want)
		}
	})

	t.Run("explicit enabled and zero values", func(t *testing.T) {
		got, err := ParseBreakerDefaults(`{"enabled":true,"model_failure_threshold":0,"upstream_threshold_logic":"and"}`)
		if err != nil {
			t.Fatalf("ParseBreakerDefaults() error = %v", err)
		}
		if !got.Enabled || got.ModelFailureThreshold != domainchannel.DefaultBreakerDefaults().ModelFailureThreshold || got.UpstreamThresholdLogic != "and" {
			t.Fatalf("ParseBreakerDefaults() = %#v", got)
		}
	})

	for _, value := range []string{
		`null`,
		`[]`,
		`{"enabled":null}`,
		`{"model_failure_threshold":null}`,
		`{"model_failure_threshold":-1}`,
		`{"upstream_threshold_logic":"xor"}`,
	} {
		t.Run(value, func(t *testing.T) {
			_, err := ParseBreakerDefaults(value)
			if !errors.Is(err, ErrInvalidBreakerDefaults) {
				t.Fatalf("ParseBreakerDefaults(%s) error = %v, want ErrInvalidBreakerDefaults", value, err)
			}
		})
	}
}

func TestMarshalBreakerDefaultsRoundTrip(t *testing.T) {
	want := domainchannel.DefaultBreakerDefaults()
	want.Enabled = true
	encoded, err := MarshalBreakerDefaults(want)
	if err != nil {
		t.Fatalf("MarshalBreakerDefaults() error = %v", err)
	}
	got, err := ParseBreakerDefaults(encoded)
	if err != nil {
		t.Fatalf("ParseBreakerDefaults() error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip = %#v, want %#v", got, want)
	}
}
