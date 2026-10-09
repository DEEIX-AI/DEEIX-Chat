package personalprovider

import (
	"context"
	"errors"
	"testing"

	domainpersonalprovider "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/personalprovider"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openPersonalProviderTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.LLMUserProvider{}, &models.User{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return db
}

func createProvider(t *testing.T, repo *Repo, owner uint, publicID string, host string) *domainpersonalprovider.Provider {
	t.Helper()
	item, err := repo.Create(context.Background(), &domainpersonalprovider.Provider{
		PublicID:    publicID,
		OwnerUserID: owner,
		Name:        publicID,
		Protocol:    "openai_chat_completions",
		BaseURL:     "https://" + host + "/v1",
		Host:        host,
		APIKeyEnc:   "v1:cipher",
		KeyHint:     "sk-••••1234",
		Models:      []domainpersonalprovider.Model{{Name: "gpt-4o", Protocols: []string{"openai_chat_completions"}}},
		Status:      domainpersonalprovider.StatusActive,
		Source:      domainpersonalprovider.SourceManual,
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	return item
}

func TestOwnerScopedAccessHidesOtherUsersProviders(t *testing.T) {
	repo := NewRepo(openPersonalProviderTestDB(t))
	ctx := context.Background()
	createProvider(t, repo, 1, "aaaaaaaa1111", "api.one.com")
	createProvider(t, repo, 2, "bbbbbbbb2222", "api.two.com")

	if _, err := repo.GetByOwner(ctx, 1, "bbbbbbbb2222"); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("user 1 reading user 2's provider: err = %v, want ErrNotFound", err)
	}
	name := "hijacked"
	if _, err := repo.UpdateByOwner(ctx, 1, "bbbbbbbb2222", repository.PersonalProviderPatch{Name: &name}); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("user 1 updating user 2's provider: err = %v, want ErrNotFound", err)
	}
	if err := repo.DeleteByOwner(ctx, 1, "bbbbbbbb2222"); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("user 1 deleting user 2's provider: err = %v, want ErrNotFound", err)
	}
	other, err := repo.GetByOwner(ctx, 2, "bbbbbbbb2222")
	if err != nil || other.Name != "bbbbbbbb2222" {
		t.Fatalf("user 2's provider changed: %#v, %v", other, err)
	}
	list, err := repo.ListByOwner(ctx, 1)
	if err != nil || len(list) != 1 || list[0].PublicID != "aaaaaaaa1111" {
		t.Fatalf("user 1 list = %#v, %v", list, err)
	}
}

func TestUpdateByOwnerPersistsModelsAndCheckState(t *testing.T) {
	repo := NewRepo(openPersonalProviderTestDB(t))
	ctx := context.Background()
	createProvider(t, repo, 1, "aaaaaaaa1111", "api.one.com")

	models := []domainpersonalprovider.Model{
		{Name: "gpt-4o", Protocols: []string{"openai_chat_completions"}},
		{Name: "gpt-image-1", Protocols: []string{"openai_image_generations", "openai_image_edits"}},
	}
	lastError := "upstream 401"
	updated, err := repo.UpdateByOwner(ctx, 1, "aaaaaaaa1111", repository.PersonalProviderPatch{Models: &models, LastError: &lastError})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if len(updated.Models) != 2 || updated.Models[1].Name != "gpt-image-1" || len(updated.Models[1].Protocols) != 2 || updated.LastError != lastError {
		t.Fatalf("updated = %#v", updated)
	}
}

// 早期版本的 models_json 只有模型名：读取时按服务协议补齐，无需迁移。
func TestLegacyModelNamesTakeTheProviderProtocol(t *testing.T) {
	db := openPersonalProviderTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()
	createProvider(t, repo, 1, "aaaaaaaa1111", "api.one.com")
	if err := db.Model(&models.LLMUserProvider{}).Where("public_id = ?", "aaaaaaaa1111").
		Update("models_json", `["gpt-4o","o4-mini"]`).Error; err != nil {
		t.Fatal(err)
	}
	item, err := repo.GetByOwner(ctx, 1, "aaaaaaaa1111")
	if err != nil {
		t.Fatal(err)
	}
	if len(item.Models) != 2 || item.Models[1].Name != "o4-mini" || len(item.Models[1].Protocols) != 1 || item.Models[1].Protocols[0] != "openai_chat_completions" {
		t.Fatalf("models = %#v", item.Models)
	}
}

func TestAdminBulkStatusByHost(t *testing.T) {
	repo := NewRepo(openPersonalProviderTestDB(t))
	ctx := context.Background()
	createProvider(t, repo, 1, "aaaaaaaa1111", "evil.example.com")
	createProvider(t, repo, 2, "bbbbbbbb2222", "evil.example.com")
	createProvider(t, repo, 3, "cccccccc3333", "good.example.com")

	affected, err := repo.SetStatusByHost(ctx, "EVIL.example.com ", domainpersonalprovider.StatusSuspended)
	if err != nil || affected != 2 {
		t.Fatalf("suspend by host affected %d, %v", affected, err)
	}
	items, total, err := repo.ListForAdmin(ctx, repository.PersonalProviderAdminFilter{Status: domainpersonalprovider.StatusSuspended}, 0, 20)
	if err != nil || total != 2 || len(items) != 2 {
		t.Fatalf("suspended list = %d/%d, %v", len(items), total, err)
	}
	good, err := repo.GetByOwner(ctx, 3, "cccccccc3333")
	if err != nil || good.Status != domainpersonalprovider.StatusActive {
		t.Fatalf("unrelated host changed: %#v, %v", good, err)
	}
}

func TestAdminSearchMatchesTheOwner(t *testing.T) {
	db := openPersonalProviderTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()
	users := []models.User{
		{BaseModel: models.BaseModel{ID: 1}, Username: "alice-ff8d9819", DisplayName: "Alice Chen", Email: "alice@example.com", PublicID: "u1"},
		{BaseModel: models.BaseModel{ID: 2}, Username: "bob", DisplayName: "", Email: "bob@corp.test", PublicID: "u2"},
	}
	for i := range users {
		if err := db.Create(&users[i]).Error; err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}
	createProvider(t, repo, 1, "aaaaaaaa1111", "openrouter.ai")
	createProvider(t, repo, 2, "bbbbbbbb2222", "api.deepseek.com")

	for query, want := range map[string]string{
		"alice chen":   "aaaaaaaa1111",
		"CORP.TEST":    "bbbbbbbb2222",
		"bob":          "bbbbbbbb2222",
		"deepseek":     "bbbbbbbb2222",
		"aaaaaaaa1111": "aaaaaaaa1111",
	} {
		items, total, err := repo.ListForAdmin(ctx, repository.PersonalProviderAdminFilter{Query: query}, 0, 20)
		if err != nil || total != 1 || len(items) != 1 || items[0].PublicID != want {
			t.Fatalf("search %q = %d items (%v), want %s", query, total, err, want)
		}
	}
}

func TestIconRoundTrip(t *testing.T) {
	repo := NewRepo(openPersonalProviderTestDB(t))
	ctx := context.Background()
	item := createProvider(t, repo, 1, "aaaaaaaa1111", "openrouter.ai")

	icon := "openrouter"
	updated, err := repo.UpdateByOwner(ctx, 1, item.PublicID, repository.PersonalProviderPatch{Icon: &icon})
	if err != nil || updated.Icon != "openrouter" {
		t.Fatalf("update icon = %#v, %v", updated, err)
	}
	reread, err := repo.GetByOwner(ctx, 1, item.PublicID)
	if err != nil || reread.Icon != "openrouter" {
		t.Fatalf("reread icon = %#v, %v", reread, err)
	}
}
