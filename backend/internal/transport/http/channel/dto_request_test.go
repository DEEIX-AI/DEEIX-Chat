package channel

import (
	"testing"

	"github.com/gin-gonic/gin/binding"
)

func TestUpsertUpstreamModelRequestRequiresCompleteProtocolSet(t *testing.T) {
	emptyProtocols := []string{}
	validProtocols := []string{"openai_responses"}
	duplicateProtocols := []string{"openai_responses", "openai_responses"}
	tooManyProtocols := []string{"openai_responses", "openai_image_generations", "openai_image_edits"}
	blankProtocol := []string{""}

	tests := []struct {
		name      string
		protocols *[]string
		wantError bool
	}{
		{name: "missing", protocols: nil, wantError: true},
		{name: "explicit defaults", protocols: &emptyProtocols},
		{name: "explicit protocol", protocols: &validProtocols},
		{name: "duplicate", protocols: &duplicateProtocols, wantError: true},
		{name: "too many", protocols: &tooManyProtocols, wantError: true},
		{name: "blank", protocols: &blankProtocol, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := UpsertUpstreamModelRequest{
				PlatformModelName: "test-model",
				UpstreamModelName: "test-model",
				Protocols:         test.protocols,
			}
			err := binding.Validator.ValidateStruct(request)
			if test.wantError && err == nil {
				t.Fatal("expected validation error")
			}
			if !test.wantError && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestSetModelProtocolsRequestRequiresExplicitNonEmptyProtocolSet(t *testing.T) {
	tests := []struct {
		name      string
		request   SetModelProtocolsRequest
		wantError bool
	}{
		{name: "valid", request: SetModelProtocolsRequest{Protocols: []string{"openai_responses"}, KindsJSON: `["chat"]`}},
		{name: "missing protocols", request: SetModelProtocolsRequest{KindsJSON: `["chat"]`}, wantError: true},
		{name: "duplicate protocols", request: SetModelProtocolsRequest{Protocols: []string{"openai_responses", "openai_responses"}, KindsJSON: `["chat"]`}, wantError: true},
		{name: "missing kinds", request: SetModelProtocolsRequest{Protocols: []string{"openai_responses"}}, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := binding.Validator.ValidateStruct(test.request)
			if test.wantError && err == nil {
				t.Fatal("expected validation error")
			}
			if !test.wantError && err != nil {
				t.Fatalf("unexpected validation error: %v", err)
			}
		})
	}
}
