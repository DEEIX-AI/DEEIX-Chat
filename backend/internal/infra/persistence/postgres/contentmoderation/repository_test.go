package contentmoderation

import (
	"context"
	"strconv"
	"testing"
	"time"

	domaincm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/contentmoderation"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestApplyRunBlockWithdrawsAssistantAttachments(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:content_moderation_apply_block?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.Message{},
		&model.Attachment{},
		&model.FileObject{},
		&model.ConversationRun{},
		&model.ChatRunEvent{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	run := model.ConversationRun{RunID: "run_with_attachment", Status: "success", ModerationState: domaincm.ModerationStateModerating, StartedAt: time.Now()}
	if err := db.Create(&run).Error; err != nil {
		t.Fatalf("create run: %v", err)
	}
	message := model.Message{RunID: run.RunID, Role: "assistant", ContentType: "image", Content: "unsafe output", ReasoningContent: "unsafe reasoning", Status: "success"}
	if err := db.Create(&message).Error; err != nil {
		t.Fatalf("create message: %v", err)
	}
	file := model.FileObject{FileID: "file_generated", UserID: 42, StoragePath: "generated/file.png", Status: "active"}
	if err := db.Create(&file).Error; err != nil {
		t.Fatalf("create file: %v", err)
	}
	attachment := model.Attachment{MessageID: message.ID, UserID: 42, FileID: file.FileID, Kind: "image", Status: "active", UploadedAt: time.Now()}
	if err := db.Create(&attachment).Error; err != nil {
		t.Fatalf("create attachment: %v", err)
	}
	trace := model.ChatRunEvent{RunID: run.RunID, EventScope: "trace_event", EventID: "trace_1", StartedAt: time.Now()}
	if err := db.Create(&trace).Error; err != nil {
		t.Fatalf("create trace: %v", err)
	}

	repo := NewRepo(db)
	fileIDs, err := repo.ApplyRunBlock(context.Background(), run.RunID, false, "cme_hit", `["violence"]`)
	if err != nil {
		t.Fatalf("apply block: %v", err)
	}
	if len(fileIDs) != 1 || fileIDs[0] != file.FileID {
		t.Fatalf("unexpected output file IDs: %#v", fileIDs)
	}

	if err := db.First(&message, message.ID).Error; err != nil {
		t.Fatalf("reload message: %v", err)
	}
	if message.Status != domaincm.StatusBlocked || message.Content != "" || message.ReasoningContent != "" {
		t.Fatalf("assistant content was not withdrawn: %#v", message)
	}
	if err := db.First(&attachment, attachment.ID).Error; err != nil {
		t.Fatalf("reload attachment: %v", err)
	}
	if attachment.Status != "deleted" {
		t.Fatalf("attachment remains accessible: %#v", attachment)
	}
	if err := db.First(&file, file.ID).Error; err != nil {
		t.Fatalf("reload file: %v", err)
	}
	if file.Status != "moderation_blocked" || file.UserID != 0 || file.StoragePath == "" {
		t.Fatalf("file was not revoked while retaining its cleanup path: %#v", file)
	}
	if err := db.First(&run, run.ID).Error; err != nil {
		t.Fatalf("reload run: %v", err)
	}
	if run.Status != domaincm.StatusBlocked || run.ModerationState != domaincm.ModerationStateBlocked {
		t.Fatalf("run was not blocked: %#v", run)
	}
	var traceCount int64
	if err := db.Model(&model.ChatRunEvent{}).Where("run_id = ?", run.RunID).Count(&traceCount).Error; err != nil {
		t.Fatalf("count traces: %v", err)
	}
	if traceCount != 0 {
		t.Fatalf("blocked trace remains visible, count=%d", traceCount)
	}
}

func TestDeleteExpiredMetadataKeepsRowsWithUnclearedContent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:content_moderation_retention?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ContentModerationEvent{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	expired := time.Now().Add(-time.Hour)
	pendingCleanup := model.ContentModerationEvent{
		PublicID:          "cme_pending_cleanup",
		ImageCount:        1,
		ImageMetaJSON:     `[{"storage_path":"moderation/pending"}]`,
		ContentExpiresAt:  expired,
		MetadataExpiresAt: expired,
	}
	cleared := model.ContentModerationEvent{
		PublicID:          "cme_cleared",
		ImageMetaJSON:     "[]",
		ContentExpiresAt:  expired,
		MetadataExpiresAt: expired,
	}
	if err := db.Create(&[]model.ContentModerationEvent{pendingCleanup, cleared}).Error; err != nil {
		t.Fatalf("create events: %v", err)
	}

	repo := NewRepo(db)
	deleted, err := repo.DeleteExpiredMetadata(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("delete expired metadata: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted=%d, want 1", deleted)
	}

	var remaining []model.ContentModerationEvent
	if err := db.Find(&remaining).Error; err != nil {
		t.Fatalf("list remaining events: %v", err)
	}
	if len(remaining) != 1 || remaining[0].PublicID != pendingCleanup.PublicID {
		t.Fatalf("uncleared isolation metadata must remain retryable: %#v", remaining)
	}
}

func TestListEventsSearchesAcrossEventAndUserMetadata(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:content_moderation_search?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.ContentModerationEvent{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	user := model.User{PublicID: "usr_reviewer", Username: "safety-admin", DisplayName: "Safety Reviewer"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	events := []model.ContentModerationEvent{
		{
			PublicID:        "cme_matching",
			UserID:          user.ID,
			RunID:           "run_alpha",
			MessagePublicID: "msg_alpha",
			Direction:       domaincm.DirectionInput,
			Modality:        domaincm.ModalityImage,
			Model:           "omni-moderation-latest",
			Result:          domaincm.ResultHit,
			ContentSummary:  "uploaded image",
		},
		{
			PublicID: "cme_other",
			UserID:   user.ID + 1,
			RunID:    "run_beta",
			Result:   domaincm.ResultFailedOpen,
		},
	}
	if err := db.Create(&events).Error; err != nil {
		t.Fatalf("create events: %v", err)
	}

	repo := NewRepo(db)
	for _, query := range []string{
		"CME_MATCHING",
		"SAFETY-ADMIN",
		"USR_REVIEWER",
		"safety reviewer",
		strconv.FormatUint(uint64(user.ID), 10),
		"run_alpha",
		"msg_alpha",
		"input",
		"image",
		"omni-moderation-latest",
		"uploaded image",
		"hit",
	} {
		items, total, listErr := repo.ListEvents(context.Background(), domaincm.EventListFilter{Query: query, Limit: 20})
		if listErr != nil {
			t.Fatalf("query %q: %v", query, listErr)
		}
		if total != 1 || len(items) != 1 || items[0].PublicID != "cme_matching" {
			t.Fatalf("query %q returned total=%d items=%#v", query, total, items)
		}
	}

	items, total, err := repo.ListEvents(context.Background(), domaincm.EventListFilter{Query: "CME_MATCH", Limit: 20})
	if err != nil {
		t.Fatalf("partial exact query: %v", err)
	}
	if total != 0 || len(items) != 0 {
		t.Fatalf("search must remain exact and indexable, total=%d items=%#v", total, items)
	}

	items, total, err = repo.ListEvents(context.Background(), domaincm.EventListFilter{Query: "%", Limit: 20})
	if err != nil {
		t.Fatalf("escaped wildcard query: %v", err)
	}
	if total != 0 || len(items) != 0 {
		t.Fatalf("wildcard must be treated literally, total=%d items=%#v", total, items)
	}
}
