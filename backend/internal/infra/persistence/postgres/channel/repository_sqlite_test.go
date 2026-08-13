package channel

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"testing"
	"time"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
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
	assertUpstreamNamesJSON(t, items[0].UpstreamNamesJSON, []string{"active-upstream"})
	assertUpstreamNamesJSON(t, items[1].UpstreamNamesJSON, []string{})

	codes, err := NewRepo(db).ListActiveRouteBindingCodesForUpstream(ctx, activeUpstream.ID)
	if err != nil {
		t.Fatalf("ListActiveRouteBindingCodesForUpstream() error = %v", err)
	}
	if !reflect.DeepEqual(codes, []string{"active-a", "active-b"}) {
		t.Fatalf("expected distinct active binding codes, got %v", codes)
	}
}

func TestModelPresentationSQLiteJoinsMetadataAndClearsDeletedGroup(t *testing.T) {
	db := openChannelSQLiteTestDB(t)
	ctx := context.Background()

	vendor := model.LLMModelVendor{Key: "acme-ai", Name: "Acme AI", Icon: "acme", SortOrder: 100}
	group := model.LLMModelDisplayGroup{Name: "Paid models", Icon: "wallet", SortOrder: 100}
	if err := db.Create(&vendor).Error; err != nil {
		t.Fatalf("create vendor: %v", err)
	}
	if err := db.Create(&group).Error; err != nil {
		t.Fatalf("create display group: %v", err)
	}
	platformModel := model.LLMPlatformModel{
		Name: "acme-pro", Vendor: vendor.Key, DisplayGroupID: &group.ID, Status: "active",
	}
	if err := db.Create(&platformModel).Error; err != nil {
		t.Fatalf("create platform model: %v", err)
	}

	repo := NewRepo(db)
	items, total, err := repo.ListModels(ctx, repository.ListChannelModelsInput{Limit: 10})
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected one model, total=%d len=%d", total, len(items))
	}
	if items[0].VendorName != vendor.Name || items[0].VendorIcon != vendor.Icon {
		t.Fatalf("unexpected vendor metadata: %#v", items[0])
	}
	if items[0].DisplayGroupName != group.Name || items[0].DisplayGroupIcon != group.Icon {
		t.Fatalf("unexpected display group metadata: %#v", items[0])
	}
	for _, query := range []string{"Acme AI", "Paid models"} {
		matched, matchedTotal, queryErr := repo.ListModels(ctx, repository.ListChannelModelsInput{Limit: 10, Query: query})
		if queryErr != nil {
			t.Fatalf("ListModels(query=%q) error = %v", query, queryErr)
		}
		if matchedTotal != 1 || len(matched) != 1 || matched[0].ID != platformModel.ID {
			t.Fatalf("expected query %q to match presentation metadata, total=%d items=%#v", query, matchedTotal, matched)
		}
	}

	if err := repo.DeleteModelDisplayGroup(ctx, group.ID); err != nil {
		t.Fatalf("DeleteModelDisplayGroup() error = %v", err)
	}
	var stored model.LLMPlatformModel
	if err := db.First(&stored, platformModel.ID).Error; err != nil {
		t.Fatalf("reload platform model: %v", err)
	}
	if stored.DisplayGroupID != nil {
		t.Fatalf("expected display group to be cleared, got %#v", stored.DisplayGroupID)
	}
	if _, err := repo.GetModelDisplayGroupByID(ctx, group.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected deleted group to be missing, got %v", err)
	}
}

func TestModelDisplayGroupSQLiteReplacesMembersAtomically(t *testing.T) {
	db := openChannelSQLiteTestDB(t)
	ctx := context.Background()
	oldGroup := model.LLMModelDisplayGroup{Name: "Old group", SortOrder: 100}
	if err := db.Create(&oldGroup).Error; err != nil {
		t.Fatalf("create old group: %v", err)
	}
	modelsToGroup := []model.LLMPlatformModel{
		{Name: "model-a", Vendor: "openai", DisplayGroupID: &oldGroup.ID, Status: "active"},
		{Name: "model-b", Vendor: "openai", Status: "active"},
		{Name: "model-c", Vendor: "openai", Status: "active"},
	}
	if err := db.Create(&modelsToGroup).Error; err != nil {
		t.Fatalf("create platform models: %v", err)
	}

	repo := NewRepo(db)
	group := domainchannel.ModelDisplayGroup{Name: "Featured"}
	if err := repo.CreateModelDisplayGroup(ctx, &group, []uint{modelsToGroup[0].ID, modelsToGroup[1].ID}); err != nil {
		t.Fatalf("CreateModelDisplayGroup() error = %v", err)
	}
	assertModelDisplayGroupID(t, db, modelsToGroup[0].ID, &group.ID)
	assertModelDisplayGroupID(t, db, modelsToGroup[1].ID, &group.ID)

	nextMembers := []uint{modelsToGroup[2].ID}
	if err := repo.UpdateModelDisplayGroup(ctx, group.ID, repository.UpdateModelDisplayGroupInput{ModelIDs: &nextMembers}); err != nil {
		t.Fatalf("UpdateModelDisplayGroup() error = %v", err)
	}
	assertModelDisplayGroupID(t, db, modelsToGroup[0].ID, nil)
	assertModelDisplayGroupID(t, db, modelsToGroup[1].ID, nil)
	assertModelDisplayGroupID(t, db, modelsToGroup[2].ID, &group.ID)

	invalidMembers := []uint{modelsToGroup[0].ID, 999999}
	if err := repo.UpdateModelDisplayGroup(ctx, group.ID, repository.UpdateModelDisplayGroupInput{ModelIDs: &invalidMembers}); !errors.Is(err, repository.ErrInvalidInput) {
		t.Fatalf("expected invalid member update to fail, got %v", err)
	}
	assertModelDisplayGroupID(t, db, modelsToGroup[0].ID, nil)
	assertModelDisplayGroupID(t, db, modelsToGroup[2].ID, &group.ID)
}

func TestModelDisplayGroupSQLiteSetsSelectedModelsAtomically(t *testing.T) {
	db := openChannelSQLiteTestDB(t)
	ctx := context.Background()
	group := model.LLMModelDisplayGroup{Name: "Featured", SortOrder: 100}
	if err := db.Create(&group).Error; err != nil {
		t.Fatalf("create display group: %v", err)
	}
	platformModels := []model.LLMPlatformModel{
		{Name: "model-a", Vendor: "openai", Status: "active"},
		{Name: "model-b", Vendor: "anthropic", Status: "active"},
	}
	if err := db.Create(&platformModels).Error; err != nil {
		t.Fatalf("create platform models: %v", err)
	}

	repo := NewRepo(db)
	modelIDs := []uint{platformModels[0].ID, platformModels[1].ID}
	if err := repo.SetModelsDisplayGroup(ctx, modelIDs, group.ID); err != nil {
		t.Fatalf("SetModelsDisplayGroup() error = %v", err)
	}
	assertModelDisplayGroupID(t, db, platformModels[0].ID, &group.ID)
	assertModelDisplayGroupID(t, db, platformModels[1].ID, &group.ID)

	if err := repo.SetModelsDisplayGroup(ctx, []uint{platformModels[0].ID}, 0); err != nil {
		t.Fatalf("clear SetModelsDisplayGroup() error = %v", err)
	}
	assertModelDisplayGroupID(t, db, platformModels[0].ID, nil)
	assertModelDisplayGroupID(t, db, platformModels[1].ID, &group.ID)

	if err := repo.SetModelsDisplayGroup(ctx, []uint{platformModels[0].ID, 999999}, group.ID); !errors.Is(err, repository.ErrInvalidInput) {
		t.Fatalf("expected invalid batch assignment to fail, got %v", err)
	}
	assertModelDisplayGroupID(t, db, platformModels[0].ID, nil)
	assertModelDisplayGroupID(t, db, platformModels[1].ID, &group.ID)
}

func TestModelVendorSQLiteAllowsDuplicateDisplayNames(t *testing.T) {
	db := openChannelSQLiteTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()

	first := domainchannel.ModelVendor{Key: "acme-primary", Name: "Acme"}
	second := domainchannel.ModelVendor{Key: "acme-secondary", Name: "Acme"}
	if err := repo.CreateModelVendor(ctx, &first); err != nil {
		t.Fatalf("create first vendor: %v", err)
	}
	if err := repo.CreateModelVendor(ctx, &second); err != nil {
		t.Fatalf("create second vendor with same display name: %v", err)
	}
}

func assertModelDisplayGroupID(t *testing.T, db *gorm.DB, modelID uint, want *uint) {
	t.Helper()
	var stored model.LLMPlatformModel
	if err := db.First(&stored, modelID).Error; err != nil {
		t.Fatalf("reload platform model %d: %v", modelID, err)
	}
	if want == nil {
		if stored.DisplayGroupID != nil {
			t.Fatalf("model %d display group = %v, want nil", modelID, *stored.DisplayGroupID)
		}
		return
	}
	if stored.DisplayGroupID == nil || *stored.DisplayGroupID != *want {
		t.Fatalf("model %d display group = %v, want %d", modelID, stored.DisplayGroupID, *want)
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

func TestListModelsSQLiteSortOrderKeepsCrossVendorDisplayGroups(t *testing.T) {
	db := openChannelSQLiteTestDB(t)
	ctx := context.Background()
	upstreamModel := createActiveRouteTarget(t, db)
	paidGroup := model.LLMModelDisplayGroup{Name: "Paid", SortOrder: 100}
	if err := db.Create(&paidGroup).Error; err != nil {
		t.Fatalf("create display group: %v", err)
	}

	models := []model.LLMPlatformModel{
		{Name: "claude-paid", Vendor: "anthropic", DisplayGroupID: &paidGroup.ID, Status: "active", SortOrder: 100},
		{Name: "gpt-free", Vendor: "openai", Status: "active", SortOrder: 200},
		{Name: "gpt-paid", Vendor: "openai", DisplayGroupID: &paidGroup.ID, Status: "active", SortOrder: 300},
		{Name: "claude-free", Vendor: "anthropic", Status: "active", SortOrder: 400},
	}
	if err := db.Create(&models).Error; err != nil {
		t.Fatalf("create platform models: %v", err)
	}
	createActiveRoutes(t, db, upstreamModel.ID, models...)

	items, _, err := NewRepo(db).ListModels(ctx, repository.ListChannelModelsInput{
		Limit: 10,
		Sort:  "sortOrder_asc",
	})
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	want := []string{"claude-paid", "gpt-paid", "gpt-free", "claude-free"}
	if got := modelNames(items); !reflect.DeepEqual(got, want) {
		t.Fatalf("expected effective display groups %v, got %v", want, got)
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

func TestListUpstreamsSQLiteCountsOnlyRouteBindings(t *testing.T) {
	db := openChannelSQLiteTestDB(t)
	ctx := context.Background()

	upstream := model.LLMUpstream{Name: "openrouter", Status: "active"}
	if err := db.Create(&upstream).Error; err != nil {
		t.Fatalf("create upstream: %v", err)
	}
	upstreamModels := []model.LLMUpstreamModel{
		{UpstreamID: upstream.ID, BindingCode: "model-a", UpstreamModelName: "model-a", Status: "active"},
		{UpstreamID: upstream.ID, BindingCode: "model-b", UpstreamModelName: "model-b", Status: "active"},
		{UpstreamID: upstream.ID, BindingCode: "model-c", UpstreamModelName: "model-c", Status: "active"},
		{UpstreamID: upstream.ID, BindingCode: "model-d", UpstreamModelName: "model-d", Status: "active"},
	}
	if err := db.Create(&upstreamModels).Error; err != nil {
		t.Fatalf("create upstream models: %v", err)
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
	if items[0].ModelsCount != 0 {
		t.Fatalf("expected unbound upstream model count 0, got %d", items[0].ModelsCount)
	}
	if items[0].ActiveModelsCount != 0 {
		t.Fatalf("expected unbound upstream active model count 0, got %d", items[0].ActiveModelsCount)
	}
}

func TestPermissionGroupDynamicModelRulesMatchCurrentModels(t *testing.T) {
	db := openChannelSQLiteTestDB(t)
	ctx := context.Background()

	groups := []model.PermissionGroup{
		{Name: "all"},
		{Name: "vendor"},
		{Name: "protocol"},
		{Name: "upstream"},
		{Name: "manual"},
		{Name: "inactive-upstream"},
	}
	if err := db.Create(&groups).Error; err != nil {
		t.Fatalf("create permission groups: %v", err)
	}

	upstreams := []model.LLMUpstream{
		{Name: "openai-upstream", Status: "active"},
		{Name: "google-upstream", Status: "active"},
		{Name: "inactive-upstream", Status: "inactive"},
	}
	if err := db.Create(&upstreams).Error; err != nil {
		t.Fatalf("create upstreams: %v", err)
	}
	upstreamModels := []model.LLMUpstreamModel{
		{UpstreamID: upstreams[0].ID, BindingCode: "openai", UpstreamModelName: "openai", Status: "active"},
		{UpstreamID: upstreams[1].ID, BindingCode: "google", UpstreamModelName: "google", Status: "active"},
		{UpstreamID: upstreams[2].ID, BindingCode: "inactive", UpstreamModelName: "inactive", Status: "active"},
	}
	if err := db.Create(&upstreamModels).Error; err != nil {
		t.Fatalf("create upstream models: %v", err)
	}
	platformModels := []model.LLMPlatformModel{
		{Name: "gpt-test", Vendor: "openai", Status: "active", SortOrder: 100},
		{Name: "gemini-test", Vendor: "google", Status: "active", SortOrder: 200},
		{Name: "claude-test", Vendor: "anthropic", Status: "active", SortOrder: 300},
	}
	if err := db.Create(&platformModels).Error; err != nil {
		t.Fatalf("create platform models: %v", err)
	}
	routes := []model.LLMPlatformModelRoute{
		{PlatformModelID: platformModels[0].ID, UpstreamModelID: upstreamModels[0].ID, Protocol: "openai_responses", Status: "active"},
		{PlatformModelID: platformModels[1].ID, UpstreamModelID: upstreamModels[1].ID, Protocol: "google_generate_content", Status: "active"},
		{PlatformModelID: platformModels[2].ID, UpstreamModelID: upstreamModels[2].ID, Protocol: "anthropic_messages", Status: "active"},
	}
	if err := db.Create(&routes).Error; err != nil {
		t.Fatalf("create routes: %v", err)
	}

	accessRows := []model.PermissionGroupModelAccess{
		{GroupID: groups[4].ID, PlatformModelID: platformModels[0].ID},
	}
	ruleRows := []model.PermissionGroupModelRule{
		{GroupID: groups[0].ID, RuleType: domainchannel.PermissionGroupModelRuleAll},
		{GroupID: groups[1].ID, RuleType: domainchannel.PermissionGroupModelRuleVendor, Value: "google"},
		{GroupID: groups[2].ID, RuleType: domainchannel.PermissionGroupModelRuleProtocol, Value: "google_generate_content"},
		{GroupID: groups[3].ID, RuleType: domainchannel.PermissionGroupModelRuleUpstream, Value: strconv.FormatUint(uint64(upstreams[1].ID), 10)},
		{GroupID: groups[5].ID, RuleType: domainchannel.PermissionGroupModelRuleUpstream, Value: strconv.FormatUint(uint64(upstreams[2].ID), 10)},
	}
	if err := db.Create(&accessRows).Error; err != nil {
		t.Fatalf("create static access rows: %v", err)
	}
	if err := db.Create(&ruleRows).Error; err != nil {
		t.Fatalf("create rule rows: %v", err)
	}

	repo := NewRepo(db)
	modelGroups, err := repo.ListModelGroupIDs(ctx, platformModels[1].ID)
	if err != nil {
		t.Fatalf("ListModelGroupIDs() error = %v", err)
	}
	wantModelGroups := []uint{groups[0].ID, groups[1].ID, groups[2].ID, groups[3].ID}
	if !reflect.DeepEqual(modelGroups, wantModelGroups) {
		t.Fatalf("expected model group IDs %v, got %v", wantModelGroups, modelGroups)
	}

	accessMap, err := repo.ListModelsWithGroupAccess(ctx)
	if err != nil {
		t.Fatalf("ListModelsWithGroupAccess() error = %v", err)
	}
	if _, ok := accessMap[platformModels[2].ID]; !ok {
		t.Fatalf("expected all-model rule to include anthropic model")
	}
	if containsUint(accessMap[platformModels[2].ID], groups[5].ID) {
		t.Fatalf("inactive upstream rule should not match active access context")
	}

	items, err := repo.ListPermissionGroups(ctx)
	if err != nil {
		t.Fatalf("ListPermissionGroups() error = %v", err)
	}
	counts := make(map[uint]int64, len(items))
	manualCounts := make(map[uint]int64, len(items))
	ruleCounts := make(map[uint]int64, len(items))
	for _, item := range items {
		counts[item.ID] = item.ModelCount
		manualCounts[item.ID] = item.ManualModelCount
		ruleCounts[item.ID] = item.RuleModelCount
	}
	if counts[groups[0].ID] != 3 {
		t.Fatalf("expected all rule count 3, got %d", counts[groups[0].ID])
	}
	if manualCounts[groups[4].ID] != 1 || ruleCounts[groups[0].ID] != 3 {
		t.Fatalf("expected manual/rule counts to be split, got manual=%v rule=%v", manualCounts, ruleCounts)
	}
	if counts[groups[1].ID] != 1 || counts[groups[2].ID] != 1 || counts[groups[3].ID] != 1 || counts[groups[4].ID] != 1 {
		t.Fatalf("expected vendor/protocol/upstream/manual counts 1, got %v", counts)
	}
	if counts[groups[5].ID] != 0 {
		t.Fatalf("expected inactive upstream rule count 0, got %d", counts[groups[5].ID])
	}
}

func TestSetModelManualGroupsDoesNotTouchDynamicRules(t *testing.T) {
	db := openChannelSQLiteTestDB(t)
	ctx := context.Background()

	groups := []model.PermissionGroup{
		{Name: "default"},
		{Name: "manual-a"},
		{Name: "manual-b"},
	}
	if err := db.Create(&groups).Error; err != nil {
		t.Fatalf("create permission groups: %v", err)
	}
	platformModel := model.LLMPlatformModel{Name: "gemini-test", Vendor: "google", Status: "active"}
	if err := db.Create(&platformModel).Error; err != nil {
		t.Fatalf("create platform model: %v", err)
	}
	if err := db.Create(&model.PermissionGroupModelRule{
		GroupID:  groups[0].ID,
		RuleType: domainchannel.PermissionGroupModelRuleVendor,
		Value:    "google",
	}).Error; err != nil {
		t.Fatalf("create model rule: %v", err)
	}
	if err := db.Create(&model.PermissionGroupModelAccess{
		GroupID:         groups[1].ID,
		PlatformModelID: platformModel.ID,
	}).Error; err != nil {
		t.Fatalf("create initial manual group: %v", err)
	}

	repo := NewRepo(db)
	if err := repo.SetModelManualGroups(ctx, platformModel.ID, []uint{groups[2].ID, groups[2].ID}); err != nil {
		t.Fatalf("SetModelManualGroups() error = %v", err)
	}

	manualIDs, err := repo.ListModelManualGroupIDs(ctx, platformModel.ID)
	if err != nil {
		t.Fatalf("ListModelManualGroupIDs() error = %v", err)
	}
	if want := []uint{groups[2].ID}; !reflect.DeepEqual(manualIDs, want) {
		t.Fatalf("expected manual group IDs %v, got %v", want, manualIDs)
	}

	matchedIDs, err := repo.ListModelGroupIDs(ctx, platformModel.ID)
	if err != nil {
		t.Fatalf("ListModelGroupIDs() error = %v", err)
	}
	if want := []uint{groups[0].ID, groups[2].ID}; !reflect.DeepEqual(matchedIDs, want) {
		t.Fatalf("expected matched group IDs %v, got %v", want, matchedIDs)
	}

	rules, err := repo.ListGroupModelRules(ctx, groups[0].ID)
	if err != nil {
		t.Fatalf("ListGroupModelRules() error = %v", err)
	}
	if len(rules) != 1 || rules[0].RuleType != domainchannel.PermissionGroupModelRuleVendor || rules[0].Value != "google" {
		t.Fatalf("expected dynamic rule to remain unchanged, got %#v", rules)
	}
}

func TestDeleteModelCascadeRemovesManualPermissionGroupAccess(t *testing.T) {
	db := openChannelSQLiteTestDB(t)
	ctx := context.Background()

	group := model.PermissionGroup{Name: "default", IsDefault: true}
	if err := db.Create(&group).Error; err != nil {
		t.Fatalf("create permission group: %v", err)
	}
	platformModel := model.LLMPlatformModel{Name: "gpt-test", Vendor: "openai", Status: "active"}
	if err := db.Create(&platformModel).Error; err != nil {
		t.Fatalf("create platform model: %v", err)
	}
	if err := db.Create(&model.PermissionGroupModelAccess{
		GroupID:         group.ID,
		PlatformModelID: platformModel.ID,
	}).Error; err != nil {
		t.Fatalf("create model access: %v", err)
	}

	if err := NewRepo(db).DeleteModelCascade(ctx, platformModel.ID); err != nil {
		t.Fatalf("DeleteModelCascade() error = %v", err)
	}

	var count int64
	if err := db.Model(&model.PermissionGroupModelAccess{}).
		Where("platform_model_id = ?", platformModel.ID).
		Count(&count).Error; err != nil {
		t.Fatalf("count model access: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected model permission group access to be deleted, got %d", count)
	}
}

func TestDeleteUpstreamCascadeRemovesUpstreamPermissionGroupRules(t *testing.T) {
	db := openChannelSQLiteTestDB(t)
	ctx := context.Background()

	group := model.PermissionGroup{Name: "default", IsDefault: true}
	if err := db.Create(&group).Error; err != nil {
		t.Fatalf("create permission group: %v", err)
	}
	upstream := model.LLMUpstream{Name: "upstream-a", Status: "active"}
	if err := db.Create(&upstream).Error; err != nil {
		t.Fatalf("create upstream: %v", err)
	}
	otherUpstream := model.LLMUpstream{Name: "upstream-b", Status: "active"}
	if err := db.Create(&otherUpstream).Error; err != nil {
		t.Fatalf("create other upstream: %v", err)
	}
	rules := []model.PermissionGroupModelRule{
		{GroupID: group.ID, RuleType: domainchannel.PermissionGroupModelRuleUpstream, Value: strconv.FormatUint(uint64(upstream.ID), 10)},
		{GroupID: group.ID, RuleType: domainchannel.PermissionGroupModelRuleUpstream, Value: strconv.FormatUint(uint64(otherUpstream.ID), 10)},
	}
	if err := db.Create(&rules).Error; err != nil {
		t.Fatalf("create rules: %v", err)
	}

	if err := NewRepo(db).DeleteUpstreamCascade(ctx, upstream.ID); err != nil {
		t.Fatalf("DeleteUpstreamCascade() error = %v", err)
	}

	var deletedRuleCount int64
	if err := db.Model(&model.PermissionGroupModelRule{}).
		Where("rule_type = ? AND value = ?", domainchannel.PermissionGroupModelRuleUpstream, strconv.FormatUint(uint64(upstream.ID), 10)).
		Count(&deletedRuleCount).Error; err != nil {
		t.Fatalf("count deleted upstream rule: %v", err)
	}
	if deletedRuleCount != 0 {
		t.Fatalf("expected deleted upstream rule to be removed, got %d", deletedRuleCount)
	}
	var remainingRuleCount int64
	if err := db.Model(&model.PermissionGroupModelRule{}).
		Where("rule_type = ? AND value = ?", domainchannel.PermissionGroupModelRuleUpstream, strconv.FormatUint(uint64(otherUpstream.ID), 10)).
		Count(&remainingRuleCount).Error; err != nil {
		t.Fatalf("count remaining upstream rule: %v", err)
	}
	if remainingRuleCount != 1 {
		t.Fatalf("expected unrelated upstream rule to remain, got %d", remainingRuleCount)
	}
}

func TestListPermissionGroupsCountsDefaultGroupUsers(t *testing.T) {
	db := openChannelSQLiteTestDB(t)
	ctx := context.Background()

	groups := []model.PermissionGroup{
		{Name: "Default", IsDefault: true},
		{Name: "Manual"},
	}
	if err := db.Create(&groups).Error; err != nil {
		t.Fatalf("create permission groups: %v", err)
	}
	users := []model.User{
		{PublicID: "user-alice", Username: "alice", DisplayName: "Alice"},
		{PublicID: "user-bob", Username: "bob", DisplayName: "Bob"},
		{PublicID: "user-cara", Username: "cara", DisplayName: "Cara"},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	if err := db.Create(&model.PermissionGroupUserAccess{
		GroupID: groups[1].ID,
		UserID:  users[0].ID,
	}).Error; err != nil {
		t.Fatalf("create user access: %v", err)
	}
	plan := model.BillingPlan{
		Code:              "pro",
		Name:              "Pro",
		IsActive:          true,
		PermissionGroupID: &groups[1].ID,
	}
	if err := db.Create(&plan).Error; err != nil {
		t.Fatalf("create billing plan: %v", err)
	}
	inactivePlan := model.BillingPlan{
		Code:              "legacy",
		Name:              "Legacy",
		IsActive:          false,
		PermissionGroupID: &groups[1].ID,
	}
	if err := db.Create(&inactivePlan).Error; err != nil {
		t.Fatalf("create inactive billing plan: %v", err)
	}
	now := time.Now()
	endAt := now.Add(time.Hour)
	if err := db.Create(&[]model.Subscription{
		{
			UserID:               users[0].ID,
			PlanID:               plan.ID,
			Status:               "active",
			StartAt:              now.Add(-time.Hour),
			CurrentPeriodStartAt: now.Add(-time.Hour),
			CurrentPeriodEndAt:   &endAt,
		},
		{
			UserID:               users[1].ID,
			PlanID:               plan.ID,
			Status:               "active",
			StartAt:              now.Add(-time.Hour),
			CurrentPeriodStartAt: now.Add(-time.Hour),
			CurrentPeriodEndAt:   &endAt,
		},
		{
			UserID:               users[2].ID,
			PlanID:               inactivePlan.ID,
			Status:               "active",
			StartAt:              now.Add(-time.Hour),
			CurrentPeriodStartAt: now.Add(-time.Hour),
			CurrentPeriodEndAt:   &endAt,
		},
	}).Error; err != nil {
		t.Fatalf("create subscriptions: %v", err)
	}

	items, err := NewRepo(db).ListPermissionGroups(ctx)
	if err != nil {
		t.Fatalf("ListPermissionGroups() error = %v", err)
	}
	userCounts := make(map[uint]int64, len(items))
	manualCounts := make(map[uint]int64, len(items))
	subscriptionCounts := make(map[uint]int64, len(items))
	for _, item := range items {
		userCounts[item.ID] = item.UserCount
		manualCounts[item.ID] = item.ManualUserCount
		subscriptionCounts[item.ID] = item.SubscriptionUserCount
	}
	if userCounts[groups[0].ID] != 3 {
		t.Fatalf("expected default group user count 3, got %d", userCounts[groups[0].ID])
	}
	if userCounts[groups[1].ID] != 2 {
		t.Fatalf("expected manual group distinct user count 2, got %d", userCounts[groups[1].ID])
	}
	if manualCounts[groups[1].ID] != 1 {
		t.Fatalf("expected manual group manual user count 1, got %d", manualCounts[groups[1].ID])
	}
	if subscriptionCounts[groups[1].ID] != 2 {
		t.Fatalf("expected manual group subscription user count 2, got %d", subscriptionCounts[groups[1].ID])
	}
}

func TestGetUserModelGroupRateMultiplierUsesMatchedModelGroups(t *testing.T) {
	db := openChannelSQLiteTestDB(t)
	ctx := context.Background()

	groups := []model.PermissionGroup{
		{Name: "Default", IsDefault: true, RateMultiplierPercent: 100},
		{Name: "Pro", RateMultiplierPercent: 80},
	}
	if err := db.Create(&groups).Error; err != nil {
		t.Fatalf("create permission groups: %v", err)
	}
	user := model.User{PublicID: "user-alice", Username: "alice", DisplayName: "Alice"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	platformModels := []model.LLMPlatformModel{
		{Name: "gpt-3", Vendor: "openai", Status: "active", SortOrder: 100},
		{Name: "gpt-4", Vendor: "openai", Status: "active", SortOrder: 200},
		{Name: "gpt-5", Vendor: "openai", Status: "active", SortOrder: 300},
		{Name: "unassigned", Vendor: "openai", Status: "active", SortOrder: 400},
	}
	if err := db.Create(&platformModels).Error; err != nil {
		t.Fatalf("create platform models: %v", err)
	}
	accessRows := []model.PermissionGroupModelAccess{
		{GroupID: groups[0].ID, PlatformModelID: platformModels[0].ID},
		{GroupID: groups[0].ID, PlatformModelID: platformModels[1].ID},
		{GroupID: groups[1].ID, PlatformModelID: platformModels[1].ID},
		{GroupID: groups[1].ID, PlatformModelID: platformModels[2].ID},
	}
	if err := db.Create(&accessRows).Error; err != nil {
		t.Fatalf("create model access rows: %v", err)
	}

	repo := NewRepo(db)
	extraGroupIDs := []uint{groups[1].ID}
	tests := []struct {
		name            string
		platformModelID uint
		want            int
	}{
		{name: "default-only model keeps default rate", platformModelID: platformModels[0].ID, want: 100},
		{name: "overlapped model uses lower matched rate", platformModelID: platformModels[1].ID, want: 80},
		{name: "pro-only model uses pro rate", platformModelID: platformModels[2].ID, want: 80},
		{name: "unassigned model ignores user group discount", platformModelID: platformModels[3].ID, want: 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.GetUserModelGroupRateMultiplierPercent(ctx, user.ID, tt.platformModelID, extraGroupIDs)
			if err != nil {
				t.Fatalf("GetUserModelGroupRateMultiplierPercent() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected rate percent %d, got %d", tt.want, got)
			}
		})
	}
}

func TestListModelsSQLiteFiltersByActiveUpstream(t *testing.T) {
	db := openChannelSQLiteTestDB(t)
	ctx := context.Background()

	upstreams := []model.LLMUpstream{
		{Name: "upstream-a", Status: "active"},
		{Name: "upstream-b", Status: "active"},
		{Name: "inactive-upstream", Status: "inactive"},
	}
	if err := db.Create(&upstreams).Error; err != nil {
		t.Fatalf("create upstreams: %v", err)
	}

	upstreamModels := []model.LLMUpstreamModel{
		{UpstreamID: upstreams[0].ID, BindingCode: "a", UpstreamModelName: "a", Status: "active"},
		{UpstreamID: upstreams[1].ID, BindingCode: "b", UpstreamModelName: "b", Status: "active"},
		{UpstreamID: upstreams[0].ID, BindingCode: "inactive-model", UpstreamModelName: "inactive-model", Status: "inactive"},
		{UpstreamID: upstreams[2].ID, BindingCode: "inactive-upstream-model", UpstreamModelName: "inactive-upstream-model", Status: "active"},
	}
	if err := db.Create(&upstreamModels).Error; err != nil {
		t.Fatalf("create upstream models: %v", err)
	}

	platformModels := []model.LLMPlatformModel{
		{Name: "a-only", Vendor: "openai", Status: "active", SortOrder: 100},
		{Name: "b-only", Vendor: "openai", Status: "active", SortOrder: 200},
		{Name: "shared", Vendor: "openai", Status: "active", SortOrder: 300},
		{Name: "inactive-route", Vendor: "openai", Status: "active", SortOrder: 400},
		{Name: "inactive-upstream-model", Vendor: "openai", Status: "active", SortOrder: 500},
		{Name: "inactive-upstream", Vendor: "openai", Status: "active", SortOrder: 600},
	}
	if err := db.Create(&platformModels).Error; err != nil {
		t.Fatalf("create platform models: %v", err)
	}

	routes := []model.LLMPlatformModelRoute{
		{PlatformModelID: platformModels[0].ID, UpstreamModelID: upstreamModels[0].ID, Protocol: "openai_responses", Status: "active"},
		{PlatformModelID: platformModels[1].ID, UpstreamModelID: upstreamModels[1].ID, Protocol: "openai_responses", Status: "active"},
		{PlatformModelID: platformModels[2].ID, UpstreamModelID: upstreamModels[0].ID, Protocol: "openai_responses", Status: "active"},
		{PlatformModelID: platformModels[2].ID, UpstreamModelID: upstreamModels[1].ID, Protocol: "openai_responses", Status: "active"},
		{PlatformModelID: platformModels[3].ID, UpstreamModelID: upstreamModels[0].ID, Protocol: "openai_responses", Status: "inactive"},
		{PlatformModelID: platformModels[4].ID, UpstreamModelID: upstreamModels[2].ID, Protocol: "openai_responses", Status: "active"},
		{PlatformModelID: platformModels[5].ID, UpstreamModelID: upstreamModels[3].ID, Protocol: "openai_responses", Status: "active"},
	}
	if err := db.Create(&routes).Error; err != nil {
		t.Fatalf("create routes: %v", err)
	}

	items, total, err := NewRepo(db).ListModels(ctx, repository.ListChannelModelsInput{
		Limit:      10,
		UpstreamID: upstreams[0].ID,
		Sort:       "platformModelName_asc",
	})
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	got := modelNames(items)
	want := []string{"a-only", "shared"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected model names %v, got %v", want, got)
	}
	assertUpstreamNamesJSON(t, items[1].UpstreamNamesJSON, []string{"upstream-a", "upstream-b"})
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
		&model.LLMModelVendor{},
		&model.LLMModelDisplayGroup{},
		&model.LLMPlatformModel{},
		&model.LLMPlatformModelRoute{},
		&model.PermissionGroup{},
		&model.PermissionGroupModelAccess{},
		&model.PermissionGroupModelRule{},
		&model.PermissionGroupUserAccess{},
		&model.User{},
		&model.BillingPlan{},
		&model.Subscription{},
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

func assertUpstreamNamesJSON(t *testing.T, raw string, expected []string) {
	t.Helper()

	var actual []string
	if err := json.Unmarshal([]byte(raw), &actual); err != nil {
		t.Fatalf("unmarshal upstream names JSON %q: %v", raw, err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("expected upstream names %v, got %v", expected, actual)
	}
}

func containsUint(items []uint, target uint) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
