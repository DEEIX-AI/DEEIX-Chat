package conversation

// ChatFilePolicyDTO 聊天文件策略响应（内部传输，不携带序列化标记）。
type ChatFilePolicyDTO struct {
	MaxMessageFiles        int
	MaxUploadFileBytes     int64
	AllowedMIMETypes       []string
	ImageMaxBytes          int64
	DocMaxBytes            int64
	EffectiveImageMaxBytes int64
	EffectiveDocMaxBytes   int64
	FullContextMaxBytes    int64
	FullContextMaxTokens   int
	FullContextPDFMaxPages int
	RAGAvailable           bool
	RAGAvailabilityReason  string
	CapabilityMode         string
	FileMode               string
}
