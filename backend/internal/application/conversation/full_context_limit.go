package conversation

import (
	"strings"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/tokenestimate"
)

// 全文注入上限
//
// 单个文件能否以全文进入对话，同时受管理员配置与当前模型的上下文预算约束：
//   - 管理员配置的大小、Token、PDF 页数上限是绝对上限（成本与延迟考虑），留空或 0 表示不限制；
//   - 启用全文注入限制时，Token 上限另取模型有效输入预算的 fullContextHardCapPercent，二者取较小值。
//     这一比例只防止单个文件撑满上下文：超出后若无法检索，文件会被整体跳过，因此取值宽松。
//
// 两个比例刻意分开：auto 文件模式在可以检索时，超过 fullContextRetrievalPercent 就改走检索，
// 改走检索几乎没有损失；全文硬上限决定的是「无法检索时是否整体跳过」，代价高得多。
const (
	fullContextRetrievalPercent = 40
	fullContextHardCapPercent   = 70
)

// 文件未纳入上下文的原因，写入 AttachmentInput.SkipReason，用于给模型的说明与处理过程展示。
const (
	fullContextSkipNoText   = "no_extracted_text"
	fullContextSkipTooLarge = "exceeds_full_context_limit"
)

// fullContextLimits 是一次请求内按当前模型解析后的全文注入上限，0 表示不限制。
type fullContextLimits struct {
	MaxBytes    int64
	MaxTokens   int64
	PDFMaxPages int
}

// resolveFullContextLimits 合并管理员绝对上限与模型上下文预算。关闭全文注入限制时，
// 运行时配置已把三项清零（见 settings.RuntimeSettings.normalizeConfig），这里也不再叠加模型上限。
func resolveFullContextLimits(cfg config.Config, modelName string, capabilitiesJSON string) fullContextLimits {
	limits := fullContextLimits{
		MaxBytes:    cfg.FileFullContextMaxBytes,
		MaxTokens:   int64(cfg.FileFullContextMaxTokens),
		PDFMaxPages: cfg.FileFullContextPDFMaxPages,
	}
	if !cfg.FileFullContextLimitEnabled {
		return limits
	}
	if modelLimit := fullContextModelTokenLimit(cfg, modelName, capabilitiesJSON, fullContextHardCapPercent); modelLimit > 0 && (limits.MaxTokens <= 0 || modelLimit < limits.MaxTokens) {
		limits.MaxTokens = modelLimit
	}
	return limits
}

// fullContextModelTokenLimit 返回当前模型有效输入预算的 percent%。
func fullContextModelTokenLimit(cfg config.Config, modelName string, capabilitiesJSON string, percent int64) int64 {
	budget := int64(domainchannel.EffectiveContextBudgetFromCapabilitiesWithFallback(modelName, capabilitiesJSON, cfg.ContextWindowFallbackTokens))
	return budget * percent / 100
}

// fullContextRejection 返回文件不能以全文注入的原因；可以注入时返回空串。
func fullContextRejection(att AttachmentInput, limits fullContextLimits) string {
	if att.FileCategory == fileCategoryVideo || att.FileCategory == fileCategoryAudio {
		return fullContextSkipNoText
	}
	text := strings.TrimSpace(att.ExtractedText)
	if text == "" {
		return fullContextSkipNoText
	}
	if limits.MaxBytes > 0 && int64(len(text)) > limits.MaxBytes {
		return fullContextSkipTooLarge
	}
	if limits.MaxTokens > 0 && tokenestimate.Estimate(text) > limits.MaxTokens {
		return fullContextSkipTooLarge
	}
	if att.FileCategory == fileCategoryPDF && limits.PDFMaxPages > 0 && att.PageCount > limits.PDFMaxPages {
		return fullContextSkipTooLarge
	}
	return ""
}

func canUseAttachmentFullContext(att AttachmentInput, limits fullContextLimits) bool {
	return fullContextRejection(att, limits) == ""
}
