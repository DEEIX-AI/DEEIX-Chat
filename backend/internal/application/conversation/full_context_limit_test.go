package conversation

import (
	"fmt"
	"strings"
	"testing"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/tokenestimate"
)

const (
	millionWindowCapabilities  = `{"contextWindow":1000000}`
	standardWindowCapabilities = `{"contextWindow":128000}`
	smallWindowCapabilities    = `{"contextWindow":32000}`
)

func fullContextLimitConfig(maxTokens int) config.Config {
	return config.Config{
		FileFullContextLimitEnabled: true,
		FileFullContextMaxTokens:    maxTokens,
		ContextWindowFallbackTokens: config.DefaultContextWindowFallbackTokens,
	}
}

func expectedModelTokenLimit(capabilitiesJSON string, percent int64) int64 {
	budget := int64(domainchannel.EffectiveContextBudgetFromCapabilitiesWithFallback("custom-model", capabilitiesJSON, config.DefaultContextWindowFallbackTokens))
	return budget * percent / 100
}

// longLogText 生成约 targetTokens 个 token 的日志文本，模拟粘贴的长日志。
func longLogText(t *testing.T, targetTokens int64) string {
	t.Helper()
	const line = "2026-10-31 12:00:00 INFO worker-3 request handled path=/api/v1/items/42 status=200\n"
	// 估算对重复文本不完全线性，按 5% 步长补足到目标规模。
	lines := int(targetTokens/tokenestimate.Estimate(line)) + 1
	text := strings.Repeat(line, lines)
	for tokenestimate.Estimate(text) < targetTokens {
		lines += lines/20 + 1
		text = strings.Repeat(line, lines)
	}
	return text
}

func TestResolveFullContextLimitsFollowsModelBudget(t *testing.T) {
	want := expectedModelTokenLimit(millionWindowCapabilities, fullContextHardCapPercent)
	if want <= 65536 {
		t.Fatalf("1M window should allow more than the legacy fixed cap, got %d", want)
	}
	limits := resolveFullContextLimits(fullContextLimitConfig(0), "custom-model", millionWindowCapabilities)
	if limits.MaxTokens != want {
		t.Fatalf("MaxTokens = %d, want model budget limit %d", limits.MaxTokens, want)
	}
}

func TestResolveFullContextLimitsDoesNotTightenLegacyCapOnStandardModels(t *testing.T) {
	// 128K 窗口与未识别模型（按 128K 回退）不能比历史固定上限 65536 更严，否则无检索时会跳过原本能注入的文件。
	for _, capabilitiesJSON := range []string{standardWindowCapabilities, ""} {
		limits := resolveFullContextLimits(fullContextLimitConfig(0), "unknown-model", capabilitiesJSON)
		if limits.MaxTokens < 65536 {
			t.Fatalf("capabilities %q: MaxTokens = %d, want at least the legacy 65536", capabilitiesJSON, limits.MaxTokens)
		}
	}
}

func TestAutoModeRetrievesBeforeHardCap(t *testing.T) {
	retrievalLimit := expectedModelTokenLimit(standardWindowCapabilities, fullContextRetrievalPercent)
	hardCap := expectedModelTokenLimit(standardWindowCapabilities, fullContextHardCapPercent)
	cfg := fullContextLimitConfig(0)
	cfg.RAGEnabled = true
	cfg.EmbeddingEnabled = true
	cfg.ContextTokenBudgetEnabled = true
	// 介于检索阈值与全文硬上限之间：能检索时改走检索，不能检索时仍以全文注入。
	attachments := []AttachmentInput{{
		FileID:        "medium",
		FileName:      "medium.log",
		FileCategory:  "text",
		Current:       true,
		EmbedStatus:   "ready",
		ExtractedText: longLogText(t, (retrievalLimit+hardCap)/2),
	}}

	plan := buildConversationFileContextPlan(attachments, "auto", cfg, "custom-model", standardWindowCapabilities, true)
	if len(plan.RAGAttachments) != 1 {
		t.Fatalf("expected retrieval when available, got %+v", plan.Attachments)
	}
	plan = buildConversationFileContextPlan(attachments, "auto", cfg, "custom-model", standardWindowCapabilities, false)
	if len(plan.FullAttachments) != 1 || plan.FullAttachments[0].ContextMode != fileContextModeFull {
		t.Fatalf("expected full context when retrieval is unavailable, got %+v", plan.Attachments)
	}
}

func TestResolveFullContextLimitsKeepsTighterAdminCap(t *testing.T) {
	limits := resolveFullContextLimits(fullContextLimitConfig(1000), "custom-model", millionWindowCapabilities)
	if limits.MaxTokens != 1000 {
		t.Fatalf("MaxTokens = %d, want admin cap 1000", limits.MaxTokens)
	}
}

func TestResolveFullContextLimitsTightensAdminCapForSmallModels(t *testing.T) {
	want := expectedModelTokenLimit(smallWindowCapabilities, fullContextHardCapPercent)
	if want >= 65536 {
		t.Fatalf("32K window should be tighter than 65536, got %d", want)
	}
	limits := resolveFullContextLimits(fullContextLimitConfig(65536), "custom-model", smallWindowCapabilities)
	if limits.MaxTokens != want {
		t.Fatalf("MaxTokens = %d, want model budget limit %d", limits.MaxTokens, want)
	}
}

func TestResolveFullContextLimitsDisabledAddsNoModelCap(t *testing.T) {
	cfg := fullContextLimitConfig(0)
	cfg.FileFullContextLimitEnabled = false
	limits := resolveFullContextLimits(cfg, "custom-model", smallWindowCapabilities)
	if limits.MaxTokens != 0 {
		t.Fatalf("MaxTokens = %d, want unlimited when full-context limits are disabled", limits.MaxTokens)
	}
}

func TestBuildFileContextPlanIncludesLongPasteOnLargeContextModel(t *testing.T) {
	// #900：约 10 万 token 的粘贴日志，模型上下文 1M，未配置检索。
	text := longLogText(t, 100_000)
	attachments := []AttachmentInput{{
		FileID:        "pasted",
		FileName:      "pasted-text.txt",
		FileCategory:  "text",
		Current:       true,
		ExtractedText: text,
	}}

	plan := buildConversationFileContextPlan(attachments, "auto", fullContextLimitConfig(0), "custom-model", millionWindowCapabilities, false)
	if len(plan.FullAttachments) != 1 || plan.FullAttachments[0].ContextMode != fileContextModeFull {
		t.Fatalf("expected long paste in full context on a 1M model, got %+v", plan.Attachments)
	}

	plan = buildConversationFileContextPlan(attachments, "auto", fullContextLimitConfig(0), "custom-model", smallWindowCapabilities, false)
	if len(plan.Skipped) != 1 || plan.Skipped[0].SkipReason != fullContextSkipTooLarge {
		t.Fatalf("expected long paste skipped as too large on a 32K model, got %+v", plan.Attachments)
	}
}

func TestBuildFileContextPlanMarksMissingTextSkipReason(t *testing.T) {
	plan := buildConversationFileContextPlan([]AttachmentInput{{
		FileID:       "empty",
		FileName:     "empty.pdf",
		FileCategory: "pdf",
		Current:      true,
	}}, "auto", fullContextLimitConfig(0), "custom-model", millionWindowCapabilities, false)
	if len(plan.Skipped) != 1 || plan.Skipped[0].SkipReason != fullContextSkipNoText {
		t.Fatalf("expected empty file skipped for missing text, got %+v", plan.Attachments)
	}
}

func TestSplitRetrievalFallbackAttachmentsRecordsSkipReason(t *testing.T) {
	_, skipped := splitRetrievalFallbackAttachments([]AttachmentInput{{
		FileID:        "large",
		FileName:      "large.md",
		FileCategory:  "document",
		ExtractedText: strings.Repeat("token ", 100),
	}}, fullContextLimits{MaxTokens: 10})
	if len(skipped) != 1 || skipped[0].SkipReason != fullContextSkipTooLarge {
		t.Fatalf("expected retrieval fallback skip reason, got %+v", skipped)
	}
}

func TestRenderTurnDocumentsExplainsSkipReason(t *testing.T) {
	text := strings.Repeat("token ", 100)
	rendered := renderTurnDocuments([]turnDocument{
		{
			Attachment: AttachmentInput{FileName: "large.log", ExtractedText: text, SkipReason: fullContextSkipTooLarge},
			Access:     turnDocumentUnavailable,
		},
		{
			Attachment: AttachmentInput{FileName: "scan.pdf", SkipReason: fullContextSkipNoText},
			Access:     turnDocumentUnavailable,
		},
	})
	tooLargeNote := fmt.Sprintf(turnDocumentTooLargeNoteFormat, tokenestimate.Estimate(text))
	if !strings.Contains(rendered, `<document source="large.log" access="unavailable">`+tooLargeNote+`</document>`) {
		t.Fatalf("expected too-large note for large.log, got %s", rendered)
	}
	if !strings.Contains(rendered, `<document source="scan.pdf" access="unavailable">`+turnDocumentUnavailableNote+`</document>`) {
		t.Fatalf("expected unavailable note for scan.pdf, got %s", rendered)
	}
}

func TestAttachmentSkipReasonSuffixCountsTooLargeFiles(t *testing.T) {
	suffix := attachmentSkipReasonSuffix([]AttachmentInput{
		{ContextMode: fileContextModeSkipped, SkipReason: fullContextSkipTooLarge},
		{ContextMode: fileContextModeSkipped, SkipReason: fullContextSkipNoText},
		{ContextMode: fileContextModeFull},
	})
	if suffix != "（1 个文件超出当前模型的全文注入上限，且无法按片段检索）" {
		t.Fatalf("suffix = %q", suffix)
	}
	if got := attachmentSkipReasonSuffix([]AttachmentInput{{ContextMode: fileContextModeSkipped, SkipReason: fullContextSkipNoText}}); got != "" {
		t.Fatalf("expected no suffix for missing-text skips, got %q", got)
	}
}
