package mcp

import (
	"errors"
	"testing"

	domainmcp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/mcp"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

func TestValidateToolAttachmentConfigAcceptsDeterministicImageBinding(t *testing.T) {
	schema := `{
		"type":"object",
		"properties":{
			"image":{"type":"string"},
			"prompt":{"type":["string","null"]}
		},
		"required":["image"]
	}`
	err := validateToolAttachmentConfig(toolAttachmentConfig{
		Mode:           domainmcp.AttachmentInputModeImage,
		Argument:       "image",
		Encoding:       domainmcp.AttachmentEncodingDataURL,
		PromptArgument: "prompt",
	}, schema)
	if err != nil {
		t.Fatalf("expected valid image attachment binding, got %v", err)
	}
}

func TestValidateToolAttachmentConfigAcceptsLocalReferencesAndUnions(t *testing.T) {
	schema := `{
		"type":"object",
		"properties":{
			"image":{"$ref":"#/$defs/image"},
			"prompt":{"anyOf":[{"$ref":"#/$defs/prompt"},{"type":"null"}]}
		},
		"$defs":{
			"image":{"type":"string"},
			"prompt":{"allOf":[{"type":"string"},{"maxLength":2000}]}
		},
		"required":["image"]
	}`
	err := validateToolAttachmentConfig(toolAttachmentConfig{
		Mode:           domainmcp.AttachmentInputModeImage,
		Argument:       "image",
		Encoding:       domainmcp.AttachmentEncodingDataURL,
		PromptArgument: "prompt",
	}, schema)
	if err != nil {
		t.Fatalf("expected referenced image attachment binding to be valid, got %v", err)
	}
}

func TestValidateToolAttachmentConfigRejectsUnresolvedReference(t *testing.T) {
	err := validateToolAttachmentConfig(toolAttachmentConfig{
		Mode:     domainmcp.AttachmentInputModeImage,
		Argument: "image",
		Encoding: domainmcp.AttachmentEncodingBase64,
	}, `{"type":"object","properties":{"image":{"$ref":"#/$defs/missing"}},"required":["image"]}`)
	if !errors.Is(err, ErrInvalidToolAttachmentConfig) {
		t.Fatalf("expected unresolved reference to be rejected, got %v", err)
	}
}

func TestPreserveCompatibleToolAttachmentConfig(t *testing.T) {
	existing := domainmcp.Tool{
		AttachmentInputMode:      domainmcp.AttachmentInputModeImage,
		AttachmentArgument:       "image",
		AttachmentEncoding:       domainmcp.AttachmentEncodingDataURL,
		AttachmentPromptArgument: "prompt",
	}
	compatible := domainmcp.Tool{InputSchemaJSON: `{
		"type":"object",
		"properties":{"image":{"type":"string"},"prompt":{"type":"string"}},
		"required":["image"]
	}`}
	preserveCompatibleToolAttachmentConfig(&compatible, existing)
	if compatible.AttachmentInputMode != domainmcp.AttachmentInputModeImage ||
		compatible.AttachmentArgument != "image" ||
		compatible.AttachmentPromptArgument != "prompt" {
		t.Fatalf("compatible attachment config was not preserved: %#v", compatible)
	}

	incompatible := domainmcp.Tool{
		InputSchemaJSON:     `{"type":"object","properties":{"file":{"type":"string"}},"required":["file"]}`,
		AttachmentInputMode: domainmcp.AttachmentInputModeNone,
	}
	preserveCompatibleToolAttachmentConfig(&incompatible, existing)
	if incompatible.AttachmentInputMode != domainmcp.AttachmentInputModeNone || incompatible.AttachmentArgument != "" {
		t.Fatalf("incompatible attachment config must be cleared: %#v", incompatible)
	}
}

func TestMergedToolAttachmentConfigClearsBindingWhenDisabled(t *testing.T) {
	mode := domainmcp.AttachmentInputModeNone
	config := mergedToolAttachmentConfig(domainmcp.Tool{
		AttachmentInputMode:      domainmcp.AttachmentInputModeImage,
		AttachmentArgument:       "image",
		AttachmentEncoding:       domainmcp.AttachmentEncodingDataURL,
		AttachmentPromptArgument: "prompt",
	}, repository.UpdateMCPToolInput{AttachmentInputMode: &mode})
	if config.Mode != domainmcp.AttachmentInputModeNone || config.Argument != "" || config.Encoding != "" || config.PromptArgument != "" {
		t.Fatalf("expected disabled attachment binding to be cleared, got %#v", config)
	}
	if err := validateToolAttachmentConfig(config, `{}`); err != nil {
		t.Fatalf("expected cleared standard tool configuration to validate, got %v", err)
	}
}

func TestValidateToolAttachmentConfigRejectsUnsupportedRequiredArguments(t *testing.T) {
	schema := `{
		"type":"object",
		"properties":{
			"image":{"type":"string"},
			"mode":{"type":"string"}
		},
		"required":["image","mode"]
	}`
	err := validateToolAttachmentConfig(toolAttachmentConfig{
		Mode:     domainmcp.AttachmentInputModeImage,
		Argument: "image",
		Encoding: domainmcp.AttachmentEncodingBase64,
	}, schema)
	if !errors.Is(err, ErrInvalidToolAttachmentConfig) {
		t.Fatalf("expected invalid attachment configuration, got %v", err)
	}
}

func TestValidateToolAttachmentConfigRejectsNonStringImageArgument(t *testing.T) {
	err := validateToolAttachmentConfig(toolAttachmentConfig{
		Mode:     domainmcp.AttachmentInputModeImage,
		Argument: "images",
		Encoding: domainmcp.AttachmentEncodingBase64,
	}, `{"type":"object","properties":{"images":{"type":"array"}}}`)
	if !errors.Is(err, ErrInvalidToolAttachmentConfig) {
		t.Fatalf("expected invalid attachment configuration, got %v", err)
	}
}
