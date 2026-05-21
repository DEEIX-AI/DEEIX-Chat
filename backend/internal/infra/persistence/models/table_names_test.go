package model

import (
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

type tableNamer interface {
	TableName() string
}

func TestTableNamesUseRestructuredDomains(t *testing.T) {
	models := []tableNamer{
		User{},
		UserContactVerification{},
		UserCredential{},
		UserSession{},
		UserAuthEvent{},
		AuthIdentityProvider{},
		UserIdentity{},
		UserTwoFactor{},
		TrustedDevice{},
		LLMUpstream{},
		LLMUpstreamModel{},
		LLMPlatformModel{},
		LLMPlatformModelRoute{},
		BillingPlan{},
		BillingPrice{},
		Subscription{},
		PaymentOrder{},
		BillingAccount{},
		BalanceTransaction{},
		ModelPricing{},
		UsageLedger{},
		Conversation{},
		ConversationShare{},
		Message{},
		ConversationMessageFeedback{},
		Attachment{},
		ConversationRun{},
		ChatRunEvent{},
		ChatContextRecord{},
		FileObject{},
		FileChunk{},
		MessageChunk{},
		UserStorageQuota{},
		MCPServer{},
		MCPTool{},
		UserMemory{},
		AuditLog{},
		SystemEvent{},
		SystemSetting{},
		UserSetting{},
	}

	deprecated := map[string]struct{}{
		"users":                             {},
		"user_contact_verifications":        {},
		"user_credentials":                  {},
		"user_sessions":                     {},
		"user_auth_events":                  {},
		"auth_identity_providers":           {},
		"user_identities":                   {},
		"user_two_factors":                  {},
		"trusted_devices":                   {},
		"user_api_keys":                     {},
		"llm_configs":                       {},
		"llm_platform_model_routes":         {},
		"billing_platform_model_prices":     {},
		"conversations":                     {},
		"messages":                          {},
		"conversation_message_feedbacks":    {},
		"attachments":                       {},
		"file_processing_results":           {},
		"user_storage_quotas":               {},
		"conversation_runs":                 {},
		"conversation_message_traces":       {},
		"conversation_message_trace_events": {},
		"conversation_tool_calls":           {},
		"conversation_context_snapshots":    {},
		"conversation_context_artifacts":    {},
		"message_chunks":                    {},
	}

	for _, item := range models {
		tableName := item.TableName()
		if _, exists := deprecated[tableName]; exists {
			t.Fatalf("model still uses deprecated table name %q", tableName)
		}
	}
}

func TestUsageLedgerColumnNames(t *testing.T) {
	parsed, err := schema.Parse(&UsageLedger{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatalf("parse usage ledger schema: %v", err)
	}

	tests := map[string]string{
		"CacheWrite5mTokens": "cache_write_5m_tokens",
		"CacheWrite1hTokens": "cache_write_1h_tokens",
	}
	for fieldName, wantColumn := range tests {
		field := parsed.LookUpField(fieldName)
		if field == nil {
			t.Fatalf("field %s not found", fieldName)
		}
		if field.DBName != wantColumn {
			t.Fatalf("field %s column = %q, want %q", fieldName, field.DBName, wantColumn)
		}
	}
}
