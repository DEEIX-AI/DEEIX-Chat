package response

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type RequestBodyErrorDetails struct {
	Reason      string                  `json:"reason"`
	Offset      int64                   `json:"offset,omitempty"`
	Field       string                  `json:"field,omitempty"`
	Expected    string                  `json:"expected,omitempty"`
	Value       string                  `json:"value,omitempty"`
	FieldErrors []RequestBodyFieldError `json:"fieldErrors,omitempty"`
}

type RequestBodyFieldError struct {
	Field   string `json:"field"`
	Rule    string `json:"rule"`
	Param   string `json:"param,omitempty"`
	Message string `json:"message"`
}

// InvalidRequestBody writes a standardized request-body validation response.
func InvalidRequestBody(c *gin.Context, err error) {
	msg, details := invalidRequestBodyMessageAndDetails(err)
	c.JSON(400, Envelope{
		ErrorMsg:  msg,
		ErrorCode: CodeRequestInvalidBody,
		Details:   details,
		RequestID: requestID(c),
		Data:      nil,
	})
}

// InvalidRequestBodyMessage converts binding and JSON errors into a user-facing
// request validation message. It is intentionally scoped to request bodies; deeper
// business validation should still be mapped by the owning handler/service.
func InvalidRequestBodyMessage(err error) string {
	msg, _ := invalidRequestBodyMessageAndDetails(err)
	return msg
}

func invalidRequestBodyMessageAndDetails(err error) (string, RequestBodyErrorDetails) {
	if err == nil {
		return "invalid request body", RequestBodyErrorDetails{Reason: "invalid"}
	}
	if errors.Is(err, io.EOF) {
		return "request body is required", RequestBodyErrorDetails{Reason: "empty_body"}
	}

	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return fmt.Sprintf("invalid JSON body at offset %d", syntaxErr.Offset), RequestBodyErrorDetails{
			Reason: "json_syntax",
			Offset: syntaxErr.Offset,
		}
	}

	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		field := requestFieldName(typeErr.Field)
		return fmt.Sprintf("%s has invalid type", field), RequestBodyErrorDetails{
			Reason:   "json_type",
			Field:    field,
			Expected: typeErr.Type.String(),
			Value:    typeErr.Value,
		}
	}

	var validationErrs validator.ValidationErrors
	if errors.As(err, &validationErrs) {
		messages := make([]string, 0, len(validationErrs))
		fieldErrors := make([]RequestBodyFieldError, 0, len(validationErrs))
		for _, fieldErr := range validationErrs {
			message := validationErrorMessage(fieldErr)
			messages = append(messages, message)
			fieldErrors = append(fieldErrors, RequestBodyFieldError{
				Field:   requestFieldName(fieldErr.Field()),
				Rule:    fieldErr.Tag(),
				Param:   fieldErr.Param(),
				Message: message,
			})
		}
		return "invalid request body: " + strings.Join(messages, "; "), RequestBodyErrorDetails{
			Reason:      "validation",
			FieldErrors: fieldErrors,
		}
	}

	return "invalid request body", RequestBodyErrorDetails{Reason: "invalid"}
}

func validationErrorMessage(fieldErr validator.FieldError) string {
	fieldName := requestFieldName(fieldErr.Field())
	switch fieldErr.Tag() {
	case "required":
		return fieldName + " is required"
	case "required_without":
		return fieldName + " is required"
	case "min":
		return fieldName + " must be at least " + fieldErr.Param()
	case "max":
		return fieldName + " must be at most " + fieldErr.Param()
	case "len":
		return fieldName + " length must be " + fieldErr.Param()
	case "oneof":
		return fieldName + " must be one of: " + fieldErr.Param()
	case "email":
		return fieldName + " must be a valid email address"
	case "url":
		return fieldName + " must be a full URL, for example https://api.example.com"
	case "uri":
		return fieldName + " must be a valid URI"
	case "gt":
		return fieldName + " must be greater than " + fieldErr.Param()
	case "gte":
		return fieldName + " must be greater than or equal to " + fieldErr.Param()
	case "lt":
		return fieldName + " must be less than " + fieldErr.Param()
	case "lte":
		return fieldName + " must be less than or equal to " + fieldErr.Param()
	default:
		return fieldName + " failed validation rule " + fieldErr.Tag()
	}
}

func requestFieldName(fieldName string) string {
	fieldName = strings.TrimSpace(fieldName)
	if fieldName == "" {
		return "field"
	}
	if strings.Contains(fieldName, ".") {
		parts := strings.Split(fieldName, ".")
		for i, part := range parts {
			parts[i] = requestFieldName(part)
		}
		return strings.Join(parts, ".")
	}
	if isLowerInitial(fieldName) {
		return fieldName
	}
	for _, initialism := range []string{"JSON", "SSO", "MCP", "API", "OTP", "URL", "URI", "USD", "CNY", "JWT", "ID"} {
		if strings.HasPrefix(fieldName, initialism) {
			rest := strings.TrimPrefix(fieldName, initialism)
			return strings.ToLower(initialism) + rest
		}
	}
	runes := []rune(fieldName)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

func isLowerInitial(value string) bool {
	for _, r := range value {
		return unicode.IsLower(r)
	}
	return false
}
