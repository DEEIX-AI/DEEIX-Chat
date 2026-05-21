package response

import (
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
)

func TestInvalidRequestBodyMessage(t *testing.T) {
	t.Run("empty body", func(t *testing.T) {
		got := InvalidRequestBodyMessage(io.EOF)
		if got != "request body is required" {
			t.Fatalf("unexpected message: %s", got)
		}
	})

	t.Run("json syntax", func(t *testing.T) {
		var payload map[string]interface{}
		err := json.Unmarshal([]byte(`{"name":`), &payload)
		got := InvalidRequestBodyMessage(err)
		if !strings.Contains(got, "invalid JSON body") {
			t.Fatalf("unexpected message: %s", got)
		}
	})

	t.Run("json type", func(t *testing.T) {
		var payload struct {
			Amount int `json:"amount"`
		}
		err := json.Unmarshal([]byte(`{"amount":"bad"}`), &payload)
		got := InvalidRequestBodyMessage(err)
		if got != "amount has invalid type" {
			t.Fatalf("unexpected message: %s", got)
		}
	})

	t.Run("validation", func(t *testing.T) {
		var payload struct {
			APIKeys []string `json:"apiKeys" validate:"required,min=1"`
			BaseURL string   `json:"baseURL" validate:"required,url"`
		}
		err := validator.New().Struct(payload)
		got := InvalidRequestBodyMessage(err)
		if !strings.Contains(got, "apiKeys is required") || !strings.Contains(got, "baseURL is required") {
			t.Fatalf("unexpected message: %s", got)
		}
	})
}
