package channel

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestListModelsSQLiteUsesPortableRouteStats(t *testing.T) {
	db := openChannelSQLiteTestDB(t)
	ctx := context.Background()

	activeUpstream := model.LLMUpstream{Name: "active-upstream", Status: "active"}
	inactiveUpstream := model.LLMUpstream{Name: "inactive-upstream", Status: "inactive"}
	if err := db.Create(&activeUpstream).Error; err != nil {
		t.Fatalf("create active upstream: %v", err)
	}
	if err := db.Create(&inactiveUpstream).Error; err != nil {
		t.Fatalf("create inactive upstream: %v", err)
	}

	upstreamModels := []model.LLMUpstreamModel{
		{UpstreamID: activeUpstream.ID, BindingCode: "active-a", UpstreamModelName: "active-a", Status: "active"},
		{UpstreamID: activeUpstream.ID, BindingCode: "active-b", UpstreamModelName: "active-b", Status: "active"},
		{UpstreamID: activeUpstream.ID, BindingCode: "inactive-model", UpstreamModelName: "inactive-model", Status: "inactive"},
		{UpstreamID: inactiveUpstream.ID, BindingCode: "inactive-upstream-model", UpstreamModelName: "inactive-upstream-model", Status: "active"},
	}
	if err := db.Create(&upstreamModels).Error; err != nil {
		t.Fatalf("create upstream models: %v", err)
	}
	activeModelA := upstreamModels[0]
	activeModelB := upstreamModels[1]
	inactiveModel := upstreamModels[2]
	inactiveUpstreamModel := upstreamModels[3]

	platformModel := model.LLMPlatformModel{Name: "gpt-test", Vendor: "openai", Status: "active", SortOrder: 1}
	emptyPlatformModel := model.LLMPlatformModel{Name: "empty-test", Vendor: "openai", Status: "active", SortOrder: 2}
	if err := db.Create(&platformModel).Error; err != nil {
		t.Fatalf("create platform model: %v", err)
	}
	if err := db.Create(&emptyPlatformModel).Error; err != nil {
		t.Fatalf("create empty platform model: %v", err)
	}

	routes := []model.LLMPlatformModelRoute{
		{PlatformModelID: platformModel.ID, UpstreamModelID: activeModelA.ID, Protocol: "openai_responses", Status: "active"},
		{PlatformModelID: platformModel.ID, UpstreamModelID: activeModelB.ID, Protocol: "openai_responses", Status: "active"},
		{PlatformModelID: platformModel.ID, UpstreamModelID: activeModelA.ID, Protocol: "xai_responses", Status: "active"},
		{PlatformModelID: platformModel.ID, UpstreamModelID: inactiveModel.ID, Protocol: "anthropic_messages", Status: "active"},
		{PlatformModelID: platformModel.ID, UpstreamModelID: inactiveUpstreamModel.ID, Protocol: "google_generate_content", Status: "active"},
		{PlatformModelID: platformModel.ID, UpstreamModelID: activeModelB.ID, Protocol: "disabled_protocol", Status: "inactive"},
	}
	if err := db.Create(&routes).Error; err != nil {
		t.Fatalf("create routes: %v", err)
	}

	items, total, err := NewRepo(db).ListModels(ctx, repository.ListChannelModelsInput{
		Limit: 10,
		Sort:  "sortOrder_asc",
	})
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].SourceCount != 6 {
		t.Fatalf("expected source count 6, got %d", items[0].SourceCount)
	}
	if items[0].ActiveSourceCount != 3 {
		t.Fatalf("expected active source count 3, got %d", items[0].ActiveSourceCount)
	}
	assertProtocolsJSON(t, items[0].ProtocolsJSON, []string{"openai_responses", "xai_responses"})
	assertProtocolsJSON(t, items[1].ProtocolsJSON, []string{})

	codes, err := NewRepo(db).ListActiveRouteBindingCodesForUpstream(ctx, activeUpstream.ID)
	if err != nil {
		t.Fatalf("ListActiveRouteBindingCodesForUpstream() error = %v", err)
	}
	if !reflect.DeepEqual(codes, []string{"active-a", "active-b"}) {
		t.Fatalf("expected distinct active binding codes, got %v", codes)
	}
}

func TestListUpstreamsSQLiteExcludesInactiveUpstreamFromActiveModelCount(t *testing.T) {
	db := openChannelSQLiteTestDB(t)
	ctx := context.Background()

	inactiveUpstream := model.LLMUpstream{Name: "inactive-upstream", Status: "inactive"}
	if err := db.Create(&inactiveUpstream).Error; err != nil {
		t.Fatalf("create inactive upstream: %v", err)
	}
	upstreamModels := []model.LLMUpstreamModel{
		{UpstreamID: inactiveUpstream.ID, BindingCode: "model-a", UpstreamModelName: "model-a", Status: "active"},
		{UpstreamID: inactiveUpstream.ID, BindingCode: "model-b", UpstreamModelName: "model-b", Status: "active"},
	}
	if err := db.Create(&upstreamModels).Error; err != nil {
		t.Fatalf("create upstream models: %v", err)
	}
	platformModel := model.LLMPlatformModel{Name: "gpt-test", Vendor: "openai", Status: "active", SortOrder: 1}
	if err := db.Create(&platformModel).Error; err != nil {
		t.Fatalf("create platform model: %v", err)
	}
	routes := []model.LLMPlatformModelRoute{
		{PlatformModelID: platformModel.ID, UpstreamModelID: upstreamModels[0].ID, Protocol: "openai_responses", Status: "active"},
		{PlatformModelID: platformModel.ID, UpstreamModelID: upstreamModels[1].ID, Protocol: "openai_responses", Status: "active"},
	}
	if err := db.Create(&routes).Error; err != nil {
		t.Fatalf("create routes: %v", err)
	}

	items, _, err := NewRepo(db).ListUpstreams(ctx, repository.ListChannelUpstreamsInput{
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("ListUpstreams() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 upstream, got %d", len(items))
	}
	if items[0].ModelsCount != 2 {
		t.Fatalf("expected total model count 2, got %d", items[0].ModelsCount)
	}
	if items[0].ActiveModelsCount != 0 {
		t.Fatalf("expected inactive upstream active model count 0, got %d", items[0].ActiveModelsCount)
	}
}

func TestListUpstreamsSQLiteExcludesInactivePlatformModelFromActiveModelCount(t *testing.T) {
	db := openChannelSQLiteTestDB(t)
	ctx := context.Background()

	upstream := model.LLMUpstream{Name: "openrouter", Status: "active"}
	if err := db.Create(&upstream).Error; err != nil {
		t.Fatalf("create upstream: %v", err)
	}
	upstreamModels := []model.LLMUpstreamModel{
		{UpstreamID: upstream.ID, BindingCode: "model-a", UpstreamModelName: "model-a", Status: "active"},
		{UpstreamID: upstream.ID, BindingCode: "model-b", UpstreamModelName: "model-b", Status: "active"},
	}
	if err := db.Create(&upstreamModels).Error; err != nil {
		t.Fatalf("create upstream models: %v", err)
	}
	platformModels := []model.LLMPlatformModel{
		{Name: "model-a", Vendor: "openai", Status: "inactive", SortOrder: 1},
		{Name: "model-b", Vendor: "openai", Status: "inactive", SortOrder: 2},
	}
	if err := db.Create(&platformModels).Error; err != nil {
		t.Fatalf("create platform models: %v", err)
	}
	routes := []model.LLMPlatformModelRoute{
		{PlatformModelID: platformModels[0].ID, UpstreamModelID: upstreamModels[0].ID, Protocol: "openai_responses", Status: "active"},
		{PlatformModelID: platformModels[1].ID, UpstreamModelID: upstreamModels[1].ID, Protocol: "openai_responses", Status: "active"},
	}
	if err := db.Create(&routes).Error; err != nil {
		t.Fatalf("create routes: %v", err)
	}

	items, _, err := NewRepo(db).ListUpstreams(ctx, repository.ListChannelUpstreamsInput{
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("ListUpstreams() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 upstream, got %d", len(items))
	}
	if items[0].ModelsCount != 2 {
		t.Fatalf("expected total model count 2, got %d", items[0].ModelsCount)
	}
	if items[0].ActiveModelsCount != 0 {
		t.Fatalf("expected inactive platform models to produce active model count 0, got %d", items[0].ActiveModelsCount)
	}
	codes, err := NewRepo(db).ListActiveRouteBindingCodesForUpstream(ctx, upstream.ID)
	if err != nil {
		t.Fatalf("ListActiveRouteBindingCodesForUpstream() error = %v", err)
	}
	if len(codes) != 0 {
		t.Fatalf("expected inactive platform models to be excluded from active binding codes, got %v", codes)
	}
}

func TestListModelsSQLiteSortOrderKeepsVendorGroups(t *testing.T) {
	db := openChannelSQLiteTestDB(t)
	ctx := context.Background()
	upstreamModel := createActiveRouteTarget(t, db)

	models := []model.LLMPlatformModel{
		{Name: "claude-sonnet-4.6", Vendor: "anthropic", Status: "active", SortOrder: 100},
		{Name: "gpt-5.5", Vendor: "openai", Status: "active", SortOrder: 200},
		{Name: "gemini-3.1-pro", Vendor: "google", Status: "active", SortOrder: 300},
		{Name: "grok-4.3", Vendor: "xai", Status: "active", SortOrder: 400},
		{Name: "claude-fable-5", Vendor: "anthropic", Status: "active", SortOrder: 1000},
	}
	if err := db.Create(&models).Error; err != nil {
		t.Fatalf("create platform models: %v", err)
	}
	createActiveRoutes(t, db, upstreamModel.ID, models...)

	items, total, err := NewRepo(db).ListModels(ctx, repository.ListChannelModelsInput{
		Limit: 10,
		Sort:  "sortOrder_asc",
	})
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if total != int64(len(models)) {
		t.Fatalf("expected total %d, got %d", len(models), total)
	}
	got := modelNames(items)
	want := []string{
		"claude-sonnet-4.6",
		"claude-fable-5",
		"gpt-5.5",
		"gemini-3.1-pro",
		"grok-4.3",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected model order %v, got %v", want, got)
	}
}

func TestListModelsSQLiteSortOrderIgnoresHiddenDisabledVendorAnchors(t *testing.T) {
	db := openChannelSQLiteTestDB(t)
	ctx := context.Background()
	upstreamModel := createActiveRouteTarget(t, db)

	models := []model.LLMPlatformModel{
		{Name: "claude-sonnet-4.6", Vendor: "anthropic", Status: "inactive", SortOrder: 100},
		{Name: "gpt-5.5", Vendor: "openai", Status: "active", SortOrder: 200},
		{Name: "gemini-3.1-pro", Vendor: "google", Status: "active", SortOrder: 300},
		{Name: "claude-fable-5", Vendor: "anthropic", Status: "active", SortOrder: 1000},
	}
	if err := db.Create(&models).Error; err != nil {
		t.Fatalf("create platform models: %v", err)
	}
	createActiveRoutes(t, db, upstreamModel.ID, models...)

	items, _, err := NewRepo(db).ListModels(ctx, repository.ListChannelModelsInput{
		Limit:      10,
		OnlyActive: true,
		Sort:       "sortOrder_asc",
	})
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	got := modelNames(items)
	want := []string{
		"gpt-5.5",
		"gemini-3.1-pro",
		"claude-fable-5",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected model order %v, got %v", want, got)
	}
}

func TestListModelsSQLiteSortOrderGroupsByAvailability(t *testing.T) {
	db := openChannelSQLiteTestDB(t)
	ctx := context.Background()
	upstreamModel := createActiveRouteTarget(t, db)

	models := []model.LLMPlatformModel{
		{Name: "disabled-claude", Vendor: "anthropic", Status: "inactive", SortOrder: 100},
		{Name: "unrouted-gpt", Vendor: "openai", Status: "active", SortOrder: 200},
		{Name: "available-gemini", Vendor: "google", Status: "active", SortOrder: 300},
	}
	if err := db.Create(&models).Error; err != nil {
		t.Fatalf("create platform models: %v", err)
	}
	createActiveRoutes(t, db, upstreamModel.ID, models[0], models[2])

	repo := NewRepo(db)
	items, _, err := repo.ListModels(ctx, repository.ListChannelModelsInput{
		Limit: 10,
		Sort:  "sortOrder_asc",
	})
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	got := modelNames(items)
	want := []string{
		"available-gemini",
		"disabled-claude",
		"unrouted-gpt",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected model order %v, got %v", want, got)
	}
}

func TestListModelsSQLiteOnlyAvailableReturnsPublicRoutableModels(t *testing.T) {
	db := openChannelSQLiteTestDB(t)
	ctx := context.Background()
	upstreamModel := createActiveRouteTarget(t, db)

	models := []model.LLMPlatformModel{
		{Name: "available-gpt", Vendor: "openai", AccessScope: "public", Status: "active", SortOrder: 100},
		{Name: "internal-gemini", Vendor: "google", AccessScope: "internal", Status: "active", SortOrder: 200},
		{Name: "unrouted-claude", Vendor: "anthropic", AccessScope: "public", Status: "active", SortOrder: 300},
		{Name: "disabled-grok", Vendor: "xai", AccessScope: "public", Status: "inactive", SortOrder: 400},
		{Name: "inactive-route", Vendor: "openai", AccessScope: "public", Status: "active", SortOrder: 500},
	}
	if err := db.Create(&models).Error; err != nil {
		t.Fatalf("create platform models: %v", err)
	}
	createActiveRoutes(t, db, upstreamModel.ID, models[0], models[1])
	if err := db.Create(&model.LLMPlatformModelRoute{
		PlatformModelID: models[4].ID,
		UpstreamModelID: upstreamModel.ID,
		Protocol:        "openai_responses",
		Status:          "inactive",
	}).Error; err != nil {
		t.Fatalf("create inactive route: %v", err)
	}

	items, total, err := NewRepo(db).ListModels(ctx, repository.ListChannelModelsInput{
		Limit:         10,
		OnlyAvailable: true,
		Sort:          "sortOrder_asc",
	})
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	got := modelNames(items)
	want := []string{"available-gpt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected model order %v, got %v", want, got)
	}
}

func TestReorderModelsSQLiteUpdatesSubmittedModelsOnly(t *testing.T) {
	db := openChannelSQLiteTestDB(t)
	ctx := context.Background()
	upstreamModel := createActiveRouteTarget(t, db)

	models := []model.LLMPlatformModel{
		{Name: "disabled-claude", Vendor: "anthropic", Status: "inactive", SortOrder: 100},
		{Name: "gpt-5.5", Vendor: "openai", Status: "active", SortOrder: 200},
		{Name: "gemini-3.1-pro", Vendor: "google", Status: "active", SortOrder: 300},
		{Name: "claude-fable-5", Vendor: "anthropic", Status: "active", SortOrder: 1000},
	}
	if err := db.Create(&models).Error; err != nil {
		t.Fatalf("create platform models: %v", err)
	}
	createActiveRoutes(t, db, upstreamModel.ID, models[1], models[2], models[3])

	repo := NewRepo(db)
	if err := repo.ReorderModels(ctx, []uint{models[1].ID, models[3].ID, models[2].ID}); err != nil {
		t.Fatalf("ReorderModels() error = %v", err)
	}
	items, _, err := repo.ListModels(ctx, repository.ListChannelModelsInput{
		Limit: 10,
		Sort:  "sortOrder_asc",
	})
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	got := modelNames(items)
	want := []string{
		"gpt-5.5",
		"claude-fable-5",
		"gemini-3.1-pro",
		"disabled-claude",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected model order %v, got %v", want, got)
	}
	var disabled model.LLMPlatformModel
	if err := db.First(&disabled, models[0].ID).Error; err != nil {
		t.Fatalf("load disabled model: %v", err)
	}
	if disabled.SortOrder != 100 {
		t.Fatalf("expected disabled model sort order to remain 100, got %d", disabled.SortOrder)
	}
}

func TestListActiveRoutesByModelIncludesPlatformCircuitDefaults(t *testing.T) {
	db := openChannelSQLiteTestDB(t)
	ctx := context.Background()
	upstreamModel := createActiveRouteTarget(t, db)

	platformModel := model.LLMPlatformModel{
		Name:               "gpt-circuit",
		Vendor:             "openai",
		Status:             "active",
		CbPolicyMode:       "enforced",
		CbFailureThreshold: 7,
		CbDurationMin:      8,
		CbWindowMin:        9,
	}
	if err := db.Create(&platformModel).Error; err != nil {
		t.Fatalf("create platform model: %v", err)
	}
	if err := db.Create(&model.LLMPlatformModelRoute{
		PlatformModelID:    platformModel.ID,
		UpstreamModelID:    upstreamModel.ID,
		Protocol:           "openai_responses",
		Status:             "active",
		CbFailureThreshold: 2,
		CbDurationMin:      3,
		CbWindowMin:        4,
	}).Error; err != nil {
		t.Fatalf("create route: %v", err)
	}

	rows, err := NewRepo(db).ListActiveRoutesByModel(ctx, platformModel.Name)
	if err != nil {
		t.Fatalf("ListActiveRoutesByModel() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 route, got %d", len(rows))
	}
	row := rows[0]
	if row.PlatformModelCbFailureThreshold != 7 || row.PlatformModelCbDurationMin != 8 || row.PlatformModelCbWindowMin != 9 {
		t.Fatalf("expected platform circuit defaults 7/8/9, got %d/%d/%d",
			row.PlatformModelCbFailureThreshold,
			row.PlatformModelCbDurationMin,
			row.PlatformModelCbWindowMin,
		)
	}
	if row.PlatformModelCbPolicyMode != "enforced" {
		t.Fatalf("expected platform circuit policy enforced, got %q", row.PlatformModelCbPolicyMode)
	}
	if row.ModelCbFailureThreshold != 2 || row.ModelCbDurationMin != 3 || row.ModelCbWindowMin != 4 {
		t.Fatalf("expected route circuit overrides 2/3/4, got %d/%d/%d",
			row.ModelCbFailureThreshold,
			row.ModelCbDurationMin,
			row.ModelCbWindowMin,
		)
	}
}

func openChannelSQLiteTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("resolve sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	if err := db.AutoMigrate(
		&model.LLMUpstream{},
		&model.LLMUpstreamModel{},
		&model.LLMPlatformModel{},
		&model.LLMPlatformModelRoute{},
	); err != nil {
		t.Fatalf("migrate channel tables: %v", err)
	}
	return db
}

func modelNames(items []ModelListRow) []string {
	results := make([]string, 0, len(items))
	for _, item := range items {
		results = append(results, item.PlatformModelName)
	}
	return results
}

func createActiveRouteTarget(t *testing.T, db *gorm.DB) model.LLMUpstreamModel {
	t.Helper()

	upstream := model.LLMUpstream{Name: "active-upstream", Status: "active"}
	if err := db.Create(&upstream).Error; err != nil {
		t.Fatalf("create active upstream: %v", err)
	}
	upstreamModel := model.LLMUpstreamModel{
		UpstreamID:        upstream.ID,
		BindingCode:       "active-route-target",
		UpstreamModelName: "active-route-target",
		Status:            "active",
	}
	if err := db.Create(&upstreamModel).Error; err != nil {
		t.Fatalf("create active upstream model: %v", err)
	}
	return upstreamModel
}

func createActiveRoutes(t *testing.T, db *gorm.DB, upstreamModelID uint, models ...model.LLMPlatformModel) {
	t.Helper()

	routes := make([]model.LLMPlatformModelRoute, 0, len(models))
	for _, item := range models {
		routes = append(routes, model.LLMPlatformModelRoute{
			PlatformModelID: item.ID,
			UpstreamModelID: upstreamModelID,
			Protocol:        "openai_responses",
			Status:          "active",
		})
	}
	if err := db.Create(&routes).Error; err != nil {
		t.Fatalf("create active routes: %v", err)
	}
}

func assertProtocolsJSON(t *testing.T, raw string, expected []string) {
	t.Helper()

	var actual []string
	if err := json.Unmarshal([]byte(raw), &actual); err != nil {
		t.Fatalf("unmarshal protocols JSON %q: %v", raw, err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("expected protocols %v, got %v", expected, actual)
	}
}
