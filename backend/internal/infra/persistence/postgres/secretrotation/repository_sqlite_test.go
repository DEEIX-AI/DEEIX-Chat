package secretrotation

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.LLMUpstream{}, &models.LLMUserProvider{}, &models.MCPServer{}, &models.AuthIdentityProvider{},
		&models.UserTwoFactor{}, &models.ContentModerationEvent{}, &models.SystemSetting{}, &models.RedemptionCode{},
	); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return db
}

// 每个字段都映射到真实存在的表与列，并且能分页列出、按条件替换。
func TestEverySecretFieldListsAndReplaces(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()
	for field, target := range secretColumns {
		for id, value := range map[int]string{1: "v1:a", 2: "", 3: "v1:c"} {
			columns, values := "id, "+target.column, []any{id, value}
			// 满足各表的唯一索引。
			switch target.table {
			case "llm_user_providers":
				columns += ", public_id, owner_user_id"
				values = append(values, fmt.Sprintf("p%d", id), 40+id)
			case "content_moderation_events":
				columns += ", public_id, content_expires_at, metadata_expires_at"
				values = append(values, fmt.Sprintf("p%d", id), time.Now(), time.Now())
			case "identity_providers":
				columns += ", public_id, slug, type"
				values = append(values, fmt.Sprintf("p%d", id), fmt.Sprintf("idp%d", id), "oidc")
			case "identity_mfa_settings":
				columns += ", user_id"
				values = append(values, id)
			case "system_settings":
				columns += ", namespace, key, value_type"
				values = append(values, "ns", fmt.Sprintf("k%d", id), "string")
			}
			placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(values)), ", ")
			if err := db.Exec("INSERT INTO "+target.table+" ("+columns+") VALUES ("+placeholders+")", values...).Error; err != nil {
				t.Fatalf("%s: seed: %v", field, err)
			}
		}
		page, err := repo.ListSecrets(ctx, field, 0, 1)
		if err != nil || len(page) != 1 || page[0].ID != 1 || page[0].Value != "v1:a" {
			t.Fatalf("%s: first page = %+v, %v", field, page, err)
		}
		// 绑定到记录的字段同时返回还原绑定上下文所需的列。
		if _, bound := bindingColumns[field]; bound && (page[0].OwnerUserID != 41 || page[0].PublicID != "p1") {
			t.Fatalf("%s: binding columns = %+v", field, page[0])
		}
		page, err = repo.ListSecrets(ctx, field, 1, 10)
		if err != nil || len(page) != 1 || page[0].ID != 3 {
			t.Fatalf("%s: empty values must be skipped, got %+v, %v", field, page, err)
		}
		if ok, err := repo.ReplaceSecret(ctx, field, 3, "v1:stale", "v1:new"); err != nil || ok {
			t.Fatalf("%s: replaced a value that had changed: %v %v", field, ok, err)
		}
		if ok, err := repo.ReplaceSecret(ctx, field, 3, "v1:c", "v1:new"); err != nil || !ok {
			t.Fatalf("%s: replace: %v %v", field, ok, err)
		}
		var value string
		db.Table(target.table).Select(target.column).Where("id = 3").Scan(&value)
		if value != "v1:new" {
			t.Fatalf("%s: stored %q", field, value)
		}
	}
	if _, err := repo.ListSecrets(ctx, repository.SecretFieldModerationIsolatedImages, 0, 1); err == nil {
		t.Fatal("expected an error for a field without a column")
	}
}

func TestSoftDeletedRowsAreIncluded(t *testing.T) {
	db := openTestDB(t)
	if err := db.Exec("INSERT INTO mcp_servers (id, auth_token_enc) VALUES (1, 'v1:a')").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO identity_mfa_settings (id, user_id, totp_secret_encrypted, deleted_at) VALUES (1, 7, 'v1:t', ?)", time.Now()).Error; err != nil {
		t.Fatal(err)
	}
	page, err := NewRepo(db).ListSecrets(context.Background(), repository.SecretFieldTOTPSecret, 0, 10)
	if err != nil || len(page) != 1 {
		t.Fatalf("soft-deleted rows still hold ciphertext and must be listed: %+v, %v", page, err)
	}
}

func TestListSettingSecretsOnlyReturnsRequestedKeys(t *testing.T) {
	db := openTestDB(t)
	for _, item := range []models.SystemSetting{
		{Namespace: "auth", Key: "smtp_password", Value: "v1:a", ValueType: "string"},
		{Namespace: "auth", Key: "smtp_host", Value: "smtp.example.com", ValueType: "string"},
		{Namespace: "content_moderation", Key: "api_key", Value: "v1:b", ValueType: "string"},
		{Namespace: "billing", Key: "epay_key", Value: "", ValueType: "string"},
	} {
		if err := db.Create(&item).Error; err != nil {
			t.Fatal(err)
		}
	}
	rows, err := NewRepo(db).ListSettingSecrets(context.Background(), []repository.SettingKey{
		{Namespace: "auth", Key: "smtp_password"},
		{Namespace: "content_moderation", Key: "api_key"},
		{Namespace: "billing", Key: "epay_key"},
	})
	if err != nil || len(rows) != 2 || rows[0].Key != "smtp_password" || rows[1].Key != "api_key" {
		t.Fatalf("rows = %+v, %v", rows, err)
	}
}

func TestReplaceRedemptionCodeIsConditional(t *testing.T) {
	db := openTestDB(t)
	if err := db.Exec("INSERT INTO billing_redemption_codes (id, code_hash, code_encrypted) VALUES (1, 'old-hash', 'v1:old'), (2, 'legacy-hash', '')").Error; err != nil {
		t.Fatal(err)
	}
	repo := NewRepo(db)
	ctx := context.Background()
	rows, err := repo.ListRedemptionCodes(ctx, 0, 10)
	if err != nil || len(rows) != 2 || rows[1].CodeEncrypted != "" {
		t.Fatalf("rows = %+v, %v", rows, err)
	}
	if ok, _ := repo.ReplaceRedemptionCode(ctx, 1, repository.StoredRedemptionCode{CodeHash: "other", CodeEncrypted: "v1:old"}, "new-hash", "v1:new"); ok {
		t.Fatal("replaced a code whose hash had changed")
	}
	if ok, err := repo.ReplaceRedemptionCode(ctx, 1, rows[0], "new-hash", "v1:new"); err != nil || !ok {
		t.Fatalf("replace: %v %v", ok, err)
	}
	var stored models.RedemptionCode
	db.Unscoped().First(&stored, 1)
	if stored.CodeHash != "new-hash" || stored.CodeEncrypted != "v1:new" {
		t.Fatalf("stored = %q %q", stored.CodeHash, stored.CodeEncrypted)
	}
}

func TestLatestIsolatedImageExpiry(t *testing.T) {
	db := openTestDB(t)
	repo := NewRepo(db)
	now := time.Now().UTC().Truncate(time.Second)
	if latest, err := repo.LatestIsolatedImageExpiry(context.Background(), now); err != nil || latest != nil {
		t.Fatalf("empty table: %v %v", latest, err)
	}
	later := now.Add(48 * time.Hour)
	for i, event := range []models.ContentModerationEvent{
		{PublicID: "a", Modality: "image", ImageMetaJSON: `[{"index":0}]`, ContentExpiresAt: now.Add(time.Hour), MetadataExpiresAt: later},
		{PublicID: "b", Modality: "image", ImageMetaJSON: `[{"index":0}]`, ContentExpiresAt: later, MetadataExpiresAt: later},
		{PublicID: "c", Modality: "image", ImageMetaJSON: "[]", ContentExpiresAt: later.Add(time.Hour), MetadataExpiresAt: later},
		{PublicID: "d", Modality: "text", ImageMetaJSON: "[]", ContentExpiresAt: later.Add(2 * time.Hour), MetadataExpiresAt: later},
		{PublicID: "e", Modality: "image", ImageMetaJSON: `[{"index":0}]`, ContentExpiresAt: now.Add(-time.Hour), MetadataExpiresAt: later},
	} {
		if err := db.Create(&event).Error; err != nil {
			t.Fatalf("event %d: %v", i, err)
		}
	}
	latest, err := repo.LatestIsolatedImageExpiry(context.Background(), now)
	if err != nil || latest == nil || !latest.Equal(later) {
		t.Fatalf("latest = %v, %v; want %v", latest, err, later)
	}
}
