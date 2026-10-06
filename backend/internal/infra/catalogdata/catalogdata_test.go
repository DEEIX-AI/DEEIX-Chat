package catalogdata_test

import (
	"testing"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/catalogdata"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/modelcatalog/modelsdev"
)

// 内置快照随版本发布，任何一份损坏都会让离线部署失去兜底数据，因此在测试中解码校验。
func TestBundledSnapshotsDecode(t *testing.T) {
	snapshot, err := modelsdev.BuiltinSnapshot()
	if err != nil || len(snapshot.Entries) < 1000 || snapshot.FetchedAt.IsZero() {
		t.Fatalf("bundled models.dev snapshot is unusable: entries=%d err=%v", len(snapshot.Entries), err)
	}

	pricing := appbilling.NewOfficialPricingService(nil, nil)
	if err := pricing.SetBuiltinSnapshot(catalogdata.OpenRouterPricing()); err != nil {
		t.Fatalf("bundled OpenRouter pricing snapshot is unusable: %v", err)
	}
}
