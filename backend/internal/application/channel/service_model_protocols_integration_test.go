package channel_test

import (
	"context"
	"path/filepath"
	"testing"

	appchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	channelrepo "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/channel"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestSetModelProtocolsKeepsSharedUpstreamCatalogMetadata(t *testing.T) {
	db := openModelProtocolsTestDB(t)
	var err error

	upstream := model.LLMUpstream{Name: "shared-upstream", Compatible: "openai", Status: "active"}
	if err = db.Create(&upstream).Error; err != nil {
		t.Fatalf("create upstream: %v", err)
	}
	upstreamModel := model.LLMUpstreamModel{
		UpstreamID:        upstream.ID,
		BindingCode:       "shared-upstream-model",
		UpstreamModelName: "shared-upstream-model",
		SuggestedProtocol: "openai_chat_completions",
		KindsJSON:         `["chat"]`,
		Status:            "active",
	}
	if err = db.Create(&upstreamModel).Error; err != nil {
		t.Fatalf("create upstream model: %v", err)
	}
	platformModels := []model.LLMPlatformModel{
		{Name: "platform-a", Vendor: "openai", KindsJSON: `["chat"]`, Status: "active"},
		{Name: "platform-b", Vendor: "openai", KindsJSON: `["chat"]`, Status: "active"},
	}
	if err = db.Create(&platformModels).Error; err != nil {
		t.Fatalf("create platform models: %v", err)
	}
	routes := []model.LLMPlatformModelRoute{
		{PlatformModelID: platformModels[0].ID, UpstreamModelID: upstreamModel.ID, Protocol: "openai_chat_completions", Status: "active", Priority: 1, Weight: 1},
		{PlatformModelID: platformModels[1].ID, UpstreamModelID: upstreamModel.ID, Protocol: "openai_chat_completions", Status: "active", Priority: 1, Weight: 1},
	}
	if err = db.Create(&routes).Error; err != nil {
		t.Fatalf("create shared routes: %v", err)
	}

	repo := channelrepo.NewRepo(db)
	service := appchannel.NewServiceWithRuntime(config.NewRuntime(config.Config{}), repo, repo, nil, nil)
	if _, err = service.SetModelProtocols(context.Background(), platformModels[0].ID, appchannel.SetModelProtocolsInput{
		Protocols: []string{"openai_responses"},
		KindsJSON: `["chat"]`,
	}); err != nil {
		t.Fatalf("SetModelProtocols() error = %v", err)
	}

	var storedUpstreamModel model.LLMUpstreamModel
	if err = db.First(&storedUpstreamModel, upstreamModel.ID).Error; err != nil {
		t.Fatalf("load shared upstream model: %v", err)
	}
	if storedUpstreamModel.SuggestedProtocol != upstreamModel.SuggestedProtocol || storedUpstreamModel.KindsJSON != upstreamModel.KindsJSON {
		t.Fatalf("expected shared upstream metadata to remain unchanged, got protocol=%q kinds=%q", storedUpstreamModel.SuggestedProtocol, storedUpstreamModel.KindsJSON)
	}

	var storedRoutes []model.LLMPlatformModelRoute
	if err = db.Where("upstream_model_id = ?", upstreamModel.ID).Order("platform_model_id ASC").Find(&storedRoutes).Error; err != nil {
		t.Fatalf("load shared routes: %v", err)
	}
	if len(storedRoutes) != 2 || storedRoutes[0].Protocol != "openai_responses" || storedRoutes[1].Protocol != "openai_chat_completions" {
		t.Fatalf("expected only platform A route to change, got %#v", storedRoutes)
	}
}

func TestSetModelProtocolsPreservesRetainedRouteConfiguration(t *testing.T) {
	db := openModelProtocolsTestDB(t)
	upstream := model.LLMUpstream{Name: "image-upstream", Compatible: "openai", Status: "active"}
	if err := db.Create(&upstream).Error; err != nil {
		t.Fatalf("create upstream: %v", err)
	}
	upstreamModel := model.LLMUpstreamModel{
		UpstreamID:        upstream.ID,
		BindingCode:       "image-upstream-model",
		UpstreamModelName: "image-upstream-model",
		SuggestedProtocol: "openai_image_generations",
		KindsJSON:         `["image_gen","image_edit"]`,
		Status:            "active",
	}
	if err := db.Create(&upstreamModel).Error; err != nil {
		t.Fatalf("create upstream model: %v", err)
	}
	platformModel := model.LLMPlatformModel{
		Name: "image-platform-model", Vendor: "openai", KindsJSON: `["image_gen","image_edit"]`, Status: "active",
	}
	if err := db.Create(&platformModel).Error; err != nil {
		t.Fatalf("create platform model: %v", err)
	}
	routes := []model.LLMPlatformModelRoute{
		{
			PlatformModelID: platformModel.ID, UpstreamModelID: upstreamModel.ID, Protocol: "openai_image_edits",
			Status: "active", Priority: 2, Weight: 3, Source: "sync", CbFailureThreshold: 5, CbDurationMin: 7, CbWindowMin: 11,
			HeadersJSON: `{"X-Route":"edit"}`,
		},
		{
			PlatformModelID: platformModel.ID, UpstreamModelID: upstreamModel.ID, Protocol: "openai_image_generations",
			Status: "inactive", Priority: 13, Weight: 17, Source: "manual", CbFailureThreshold: 19, CbDurationMin: 23, CbWindowMin: 29,
			HeadersJSON: `{"X-Route":"generation"}`,
		},
	}
	if err := db.Create(&routes).Error; err != nil {
		t.Fatalf("create protocol routes: %v", err)
	}

	repo := channelrepo.NewRepo(db)
	service := appchannel.NewServiceWithRuntime(config.NewRuntime(config.Config{}), repo, repo, nil, nil)
	if _, err := service.SetModelProtocols(context.Background(), platformModel.ID, appchannel.SetModelProtocolsInput{
		Protocols: []string{"openai_image_generations"},
		KindsJSON: `["image_gen"]`,
	}); err != nil {
		t.Fatalf("SetModelProtocols() error = %v", err)
	}

	var storedRoutes []model.LLMPlatformModelRoute
	if err := db.Where("platform_model_id = ? AND upstream_model_id = ?", platformModel.ID, upstreamModel.ID).Find(&storedRoutes).Error; err != nil {
		t.Fatalf("load retained route: %v", err)
	}
	if len(storedRoutes) != 1 {
		t.Fatalf("expected one retained route, got %#v", storedRoutes)
	}
	stored := storedRoutes[0]
	want := routes[1]
	if stored.ID != want.ID ||
		stored.Protocol != want.Protocol ||
		stored.Status != want.Status ||
		stored.Priority != want.Priority ||
		stored.Weight != want.Weight ||
		stored.Source != want.Source ||
		stored.CbFailureThreshold != want.CbFailureThreshold ||
		stored.CbDurationMin != want.CbDurationMin ||
		stored.CbWindowMin != want.CbWindowMin ||
		stored.HeadersJSON != want.HeadersJSON {
		t.Fatalf("expected retained route configuration to stay unchanged, got %#v", stored)
	}
}

func TestUpsertUpstreamModelPreservesRouteConfigurationAndSharedCatalog(t *testing.T) {
	db := openModelProtocolsTestDB(t)
	upstream := model.LLMUpstream{Name: "binding-upstream", Compatible: "openai", Status: "active"}
	if err := db.Create(&upstream).Error; err != nil {
		t.Fatalf("create upstream: %v", err)
	}
	upstreamModel := model.LLMUpstreamModel{
		UpstreamID:        upstream.ID,
		BindingCode:       "binding-upstream-model",
		UpstreamModelName: "binding-upstream-model",
		SuggestedProtocol: "openai_chat_completions",
		KindsJSON:         `["chat"]`,
		Status:            "active",
		Source:            "sync",
		RawJSON:           `{"ownedBy":"catalog"}`,
	}
	if err := db.Create(&upstreamModel).Error; err != nil {
		t.Fatalf("create upstream model: %v", err)
	}
	platformModels := []model.LLMPlatformModel{
		{Name: "binding-platform-a", Vendor: "openai", KindsJSON: `["image_gen","image_edit"]`, Status: "active"},
		{Name: "binding-platform-b", Vendor: "openai", KindsJSON: `["chat"]`, Status: "active"},
	}
	if err := db.Create(&platformModels).Error; err != nil {
		t.Fatalf("create platform models: %v", err)
	}
	routes := []model.LLMPlatformModelRoute{
		{
			PlatformModelID: platformModels[0].ID, UpstreamModelID: upstreamModel.ID, Protocol: "openai_image_edits",
			Status: "active", Priority: 2, Weight: 3, Source: "sync", CbFailureThreshold: 5, CbDurationMin: 7, CbWindowMin: 11,
			HeadersJSON: `{"X-Route":"edit"}`,
		},
		{
			PlatformModelID: platformModels[0].ID, UpstreamModelID: upstreamModel.ID, Protocol: "openai_image_generations",
			Status: "inactive", Priority: 13, Weight: 17, Source: "manual", CbFailureThreshold: 19, CbDurationMin: 23, CbWindowMin: 29,
			HeadersJSON: `{"X-Route":"generation"}`,
		},
		{
			PlatformModelID: platformModels[1].ID, UpstreamModelID: upstreamModel.ID, Protocol: "openai_chat_completions",
			Status: "active", Priority: 31, Weight: 37, Source: "manual",
		},
	}
	if err := db.Create(&routes).Error; err != nil {
		t.Fatalf("create routes: %v", err)
	}

	repo := channelrepo.NewRepo(db)
	service := appchannel.NewServiceWithRuntime(config.NewRuntime(config.Config{}), repo, repo, nil, nil)
	if _, err := service.UpsertUpstreamModel(context.Background(), upstream.ID, appchannel.UpsertUpstreamModelInput{
		RouteIDs:          []uint{routes[0].ID, routes[1].ID},
		PlatformModelName: platformModels[0].Name,
		UpstreamModelName: upstreamModel.UpstreamModelName,
		Protocols:         []string{"openai_image_generations", "openai_image_edits"},
		KindsJSON:         platformModels[0].KindsJSON,
	}); err != nil {
		t.Fatalf("UpsertUpstreamModel() error = %v", err)
	}

	var storedCatalog model.LLMUpstreamModel
	if err := db.First(&storedCatalog, upstreamModel.ID).Error; err != nil {
		t.Fatalf("load upstream catalog model: %v", err)
	}
	if storedCatalog.SuggestedProtocol != upstreamModel.SuggestedProtocol ||
		storedCatalog.KindsJSON != upstreamModel.KindsJSON ||
		storedCatalog.Status != upstreamModel.Status ||
		storedCatalog.Source != upstreamModel.Source ||
		storedCatalog.RawJSON != upstreamModel.RawJSON {
		t.Fatalf("expected shared upstream catalog metadata to remain unchanged, got %#v", storedCatalog)
	}

	var storedRoutes []model.LLMPlatformModelRoute
	if err := db.Where("platform_model_id = ?", platformModels[0].ID).Order("id ASC").Find(&storedRoutes).Error; err != nil {
		t.Fatalf("load preserved routes: %v", err)
	}
	if len(storedRoutes) != 2 {
		t.Fatalf("expected two preserved routes, got %#v", storedRoutes)
	}
	for index, stored := range storedRoutes {
		want := routes[index]
		if stored.ID != want.ID ||
			stored.Protocol != want.Protocol ||
			stored.Status != want.Status ||
			stored.Priority != want.Priority ||
			stored.Weight != want.Weight ||
			stored.Source != want.Source ||
			stored.CbFailureThreshold != want.CbFailureThreshold ||
			stored.CbDurationMin != want.CbDurationMin ||
			stored.CbWindowMin != want.CbWindowMin ||
			stored.HeadersJSON != want.HeadersJSON {
			t.Fatalf("expected route %d configuration to remain unchanged, got %#v", want.ID, stored)
		}
	}

	var sharedRoute model.LLMPlatformModelRoute
	if err := db.First(&sharedRoute, routes[2].ID).Error; err != nil {
		t.Fatalf("load unrelated shared route: %v", err)
	}
	if sharedRoute.Protocol != routes[2].Protocol || sharedRoute.Priority != routes[2].Priority || sharedRoute.Weight != routes[2].Weight {
		t.Fatalf("expected unrelated shared route to remain unchanged, got %#v", sharedRoute)
	}
}

func TestUpsertUpstreamModelAppliesOnlyExplicitRouteOverrides(t *testing.T) {
	db := openModelProtocolsTestDB(t)
	upstream := model.LLMUpstream{Name: "override-upstream", Compatible: "openai", Status: "active"}
	if err := db.Create(&upstream).Error; err != nil {
		t.Fatalf("create upstream: %v", err)
	}
	upstreamModel := model.LLMUpstreamModel{
		UpstreamID: upstream.ID, BindingCode: "override-model", UpstreamModelName: "override-model",
		SuggestedProtocol: "openai_image_generations", KindsJSON: `["image_gen","image_edit"]`, Status: "active",
	}
	if err := db.Create(&upstreamModel).Error; err != nil {
		t.Fatalf("create upstream model: %v", err)
	}
	platformModel := model.LLMPlatformModel{
		Name: "override-platform", Vendor: "openai", KindsJSON: `["image_gen","image_edit"]`, Status: "active",
	}
	if err := db.Create(&platformModel).Error; err != nil {
		t.Fatalf("create platform model: %v", err)
	}
	routes := []model.LLMPlatformModelRoute{
		{PlatformModelID: platformModel.ID, UpstreamModelID: upstreamModel.ID, Protocol: "openai_image_generations", Status: "active", Priority: 2, Weight: 3, Source: "sync", HeadersJSON: `{"X-Route":"generation"}`},
		{PlatformModelID: platformModel.ID, UpstreamModelID: upstreamModel.ID, Protocol: "openai_image_edits", Status: "inactive", Priority: 5, Weight: 7, Source: "manual", HeadersJSON: `{"X-Route":"edit"}`},
	}
	if err := db.Create(&routes).Error; err != nil {
		t.Fatalf("create routes: %v", err)
	}

	status := "inactive"
	repo := channelrepo.NewRepo(db)
	service := appchannel.NewServiceWithRuntime(config.NewRuntime(config.Config{}), repo, repo, nil, nil)
	if _, err := service.UpsertUpstreamModel(context.Background(), upstream.ID, appchannel.UpsertUpstreamModelInput{
		RouteIDs:          []uint{routes[0].ID, routes[1].ID},
		PlatformModelName: platformModel.Name,
		UpstreamModelName: upstreamModel.UpstreamModelName,
		Protocols:         []string{"openai_image_generations", "openai_image_edits"},
		KindsJSON:         platformModel.KindsJSON,
		Status:            &status,
	}); err != nil {
		t.Fatalf("UpsertUpstreamModel() error = %v", err)
	}

	var storedRoutes []model.LLMPlatformModelRoute
	if err := db.Where("platform_model_id = ?", platformModel.ID).Order("id ASC").Find(&storedRoutes).Error; err != nil {
		t.Fatalf("load overridden routes: %v", err)
	}
	if len(storedRoutes) != 2 {
		t.Fatalf("expected two routes, got %#v", storedRoutes)
	}
	for index, stored := range storedRoutes {
		want := routes[index]
		if stored.Status != status {
			t.Fatalf("expected explicit status override, got %#v", stored)
		}
		if stored.Priority != want.Priority || stored.Weight != want.Weight || stored.Source != want.Source || stored.HeadersJSON != want.HeadersJSON {
			t.Fatalf("expected non-overridden configuration to remain unchanged, got %#v", stored)
		}
	}
}

func openModelProtocolsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "channel.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("resolve sqlite database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err = db.AutoMigrate(
		&model.LLMUpstream{},
		&model.LLMUpstreamModel{},
		&model.LLMPlatformModel{},
		&model.LLMPlatformModelRoute{},
		&model.LLMModelVendor{},
		&model.LLMModelDisplayGroup{},
	); err != nil {
		t.Fatalf("migrate channel tables: %v", err)
	}
	return db
}
