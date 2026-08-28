// Package probe 组合各抽取引擎的端点探活与托管地址解析，实现 runtime 的 EngineProber 端口。
package probe

import (
	"context"

	docling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/extract/docling"
	mineru "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/extract/mineru"
	ocr "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/extract/ocr"
	tika "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/extract/tika"
)

// Prober 聚合各引擎的探活实现。
type Prober struct{}

// ProbeTika 探测 Tika 端点可用性。
func (Prober) ProbeTika(ctx context.Context, baseURL, authToken string) (bool, string) {
	return tika.ProbeEndpoint(ctx, baseURL, authToken)
}

// ResolveManagedTikaBaseURL 解析托管 Tika 服务地址。
func (Prober) ResolveManagedTikaBaseURL(ctx context.Context) string {
	return tika.ResolveManagedBaseURL(ctx)
}

// ProbeDocling 探测 Docling 端点可用性。
func (Prober) ProbeDocling(ctx context.Context, baseURL, authToken string) (bool, string) {
	return docling.ProbeEndpoint(ctx, baseURL, authToken)
}

// ProbeMinerU 探测 MinerU 端点可用性。
func (Prober) ProbeMinerU(ctx context.Context, baseURL, authToken string) (bool, string) {
	return mineru.ProbeEndpoint(ctx, baseURL, authToken)
}

// ProbeOCR 探测 OCR 端点可用性。
func (Prober) ProbeOCR(ctx context.Context, baseURL, authToken string) (bool, string) {
	return ocr.ProbeOCREndpoint(ctx, baseURL, authToken)
}

// ProbeRapidOCR 探测 RapidOCR 端点可用性。
func (Prober) ProbeRapidOCR(ctx context.Context, baseURL, authToken string) (bool, string) {
	return ocr.ProbeRapidOCREndpoint(ctx, baseURL, authToken)
}

// ResolveManagedRapidOCRBaseURL 解析托管 RapidOCR 服务地址。
func (Prober) ResolveManagedRapidOCRBaseURL(ctx context.Context) string {
	return ocr.ResolveManagedRapidOCRBaseURL(ctx)
}
