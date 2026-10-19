package conversation

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestSearchMessageChunksFiltersPostgresBranchBeforeTopK(t *testing.T) {
	const embeddingSignature = "test-model@1536"
	dsn := strings.TrimSpace(os.Getenv("DEEIX_TEST_DATABASE_DSN"))
	if dsn == "" {
		t.Skip("set DEEIX_TEST_DATABASE_DSN to run PostgreSQL branch-scoped vector integration test")
	}

	db, cleanup := openConversationPostgresIntegrationDB(t, dsn)
	t.Cleanup(cleanup)
	if err := db.AutoMigrate(&models.Message{}, &models.MessageChunk{}); err != nil {
		t.Fatalf("migrate conversation vector models: %v", err)
	}
	if err := db.Exec(`ALTER TABLE chat_message_chunks ADD COLUMN IF NOT EXISTS embedding vector`).Error; err != nil {
		t.Fatalf("add message embedding column: %v", err)
	}

	root := models.Message{ConversationID: 20, UserID: 1, PublicID: "msg_pg_vector_root", Role: "user", Status: "success"}
	if err := db.Create(&root).Error; err != nil {
		t.Fatalf("create root message: %v", err)
	}
	active := models.Message{
		ConversationID: 20, UserID: 1, PublicID: "msg_pg_vector_active", ParentMessageID: &root.ID,
		Role: "assistant", Status: "success",
	}
	if err := db.Create(&active).Error; err != nil {
		t.Fatalf("create active message: %v", err)
	}
	sibling := models.Message{
		ConversationID: 20, UserID: 1, PublicID: "msg_pg_vector_sibling", ParentMessageID: &root.ID,
		Role: "assistant", BranchReason: "retry", Status: "success",
	}
	if err := db.Create(&sibling).Error; err != nil {
		t.Fatalf("create sibling message: %v", err)
	}
	leaf := models.Message{
		ConversationID: 20, UserID: 1, PublicID: "msg_pg_vector_leaf", ParentMessageID: &active.ID,
		Role: "user", Status: "pending",
	}
	if err := db.Create(&leaf).Error; err != nil {
		t.Fatalf("create leaf message: %v", err)
	}

	chunks := []models.MessageChunk{
		{ConversationID: 20, MessageID: active.ID, UserID: 1, Role: "assistant", Content: "active branch target", EmbeddingSignature: embeddingSignature},
		{ConversationID: 20, MessageID: sibling.ID, UserID: 1, Role: "assistant", Content: "closer sibling target", EmbeddingSignature: embeddingSignature},
		{ConversationID: 20, MessageID: root.ID, UserID: 1, Role: "user", Content: "legacy signature target", EmbeddingSignature: "other-model@1536"},
	}
	if err := db.Create(&chunks).Error; err != nil {
		t.Fatalf("create message chunks: %v", err)
	}
	queryEmbedding := make([]float32, 1536)
	queryEmbedding[0] = 1
	activeEmbedding := make([]float32, 1536)
	activeEmbedding[0], activeEmbedding[1] = 0.8, 0.6
	activeVector, err := float32SliceToPostgresVector(activeEmbedding)
	if err != nil {
		t.Fatalf("serialize active embedding: %v", err)
	}
	queryVector, err := float32SliceToPostgresVector(queryEmbedding)
	if err != nil {
		t.Fatalf("serialize query embedding: %v", err)
	}
	if err := db.Exec(`UPDATE chat_message_chunks SET embedding = ?::vector WHERE id = ?`, activeVector, chunks[0].ID).Error; err != nil {
		t.Fatalf("write active embedding: %v", err)
	}
	if err := db.Exec(`UPDATE chat_message_chunks SET embedding = ?::vector WHERE id = ?`, queryVector, chunks[1].ID).Error; err != nil {
		t.Fatalf("write sibling embedding: %v", err)
	}
	if err := db.Exec(`UPDATE chat_message_chunks SET embedding = ?::vector WHERE id = ?`, queryVector, chunks[2].ID).Error; err != nil {
		t.Fatalf("write legacy-signature embedding: %v", err)
	}
	results, err := NewRepo(db).SearchMessageChunks(context.Background(), repository.MessageChunkSearchInput{
		Scope: repository.HistoricalMessageScope{
			ConversationID: 20,
			UserID:         1,
			LeafMessageID:  leaf.ID,
		},
		QueryEmbedding:     queryEmbedding,
		EmbeddingSignature: embeddingSignature,
		TopK:               1,
	})
	if err != nil {
		t.Fatalf("SearchMessageChunks() error = %v", err)
	}
	if len(results) != 1 || results[0].MessageID != active.ID {
		t.Fatalf("expected active branch result despite closer sibling, got %#v", results)
	}
}

func openConversationPostgresIntegrationDB(t *testing.T, dsn string) (*gorm.DB, func()) {
	t.Helper()
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("resolve postgres db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	var vectorAvailable bool
	if err := db.Raw(`SELECT to_regtype('vector') IS NOT NULL`).Scan(&vectorAvailable).Error; err != nil {
		_ = sqlDB.Close()
		t.Fatalf("check pgvector extension: %v", err)
	}
	if !vectorAvailable {
		_ = sqlDB.Close()
		t.Skip("pgvector extension is required for PostgreSQL branch-scoped vector integration test")
	}
	schemaName := fmt.Sprintf("deeix_test_conversation_scope_%d", time.Now().UnixNano())
	if err := db.Exec(`CREATE SCHEMA ` + schemaName).Error; err != nil {
		_ = sqlDB.Close()
		t.Fatalf("create test schema: %v", err)
	}
	cleanup := func() {
		_ = db.Exec(`DROP SCHEMA IF EXISTS ` + schemaName + ` CASCADE`).Error
		_ = sqlDB.Close()
	}
	if err := db.Exec(`SET search_path TO ` + schemaName + `, public`).Error; err != nil {
		cleanup()
		t.Fatalf("set test search path: %v", err)
	}
	return db, cleanup
}

// 工具抓到二进制内容时，渲染后的轨迹正文会带上 NUL 与非法 UTF-8；PostgreSQL 会拒绝整行，
// 轨迹就停在「进行中」。写入前必须清理，且后续的终态更新要能覆盖这一行。
func TestUpsertConversationTraceRowsAcceptBinaryToolOutputOnPostgres(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("DEEIX_TEST_DATABASE_DSN"))
	if dsn == "" {
		t.Skip("set DEEIX_TEST_DATABASE_DSN to run PostgreSQL trace persistence integration test")
	}

	db, cleanup := openConversationPostgresIntegrationDB(t, dsn)
	t.Cleanup(cleanup)
	if err := db.AutoMigrate(&models.ChatRunEvent{}); err != nil {
		t.Fatalf("migrate chat run events: %v", err)
	}
	repo := NewRepo(db)
	ctx := context.Background()
	binary := "**web_fetch_exa**：已完成\nGIF89a\x00\x02\x00\xff\xfe"

	event := &domainconversation.MessageTraceEventRow{
		MessageID: 1, ConversationID: 1, UserID: 1, RunID: "run_binary",
		EventID: "tools_3", EventType: "tool", Phase: "tools", Status: "streaming",
		ContentMarkdown: "**web_fetch_exa**：进行中", PayloadJSON: "{}", StartedAt: time.Now(),
	}
	if err := repo.UpsertConversationMessageTraceEvent(ctx, event); err != nil {
		t.Fatalf("insert streaming event: %v", err)
	}
	endedAt := time.Now()
	event.Status = "completed"
	event.ContentMarkdown = binary
	event.Summary = "1 次工具调用已完成\x00"
	event.EndedAt = &endedAt
	if err := repo.UpsertConversationMessageTraceEvent(ctx, event); err != nil {
		t.Fatalf("complete event with binary tool output: %v", err)
	}
	if err := repo.UpsertConversationMessageTrace(ctx, &domainconversation.MessageTrace{
		MessageID: 1, ConversationID: 1, UserID: 1, RunID: "run_binary", TraceType: "tools",
		Status: "completed", ContentMarkdown: binary, PayloadJSON: "{}", StartedAt: time.Now(), EndedAt: &endedAt,
	}); err != nil {
		t.Fatalf("upsert trace block with binary tool output: %v", err)
	}
	if err := repo.CreateConversationToolCall(ctx, &domainconversation.ToolCall{
		MessageID: 1, ConversationID: 1, UserID: 1, RunID: "run_binary", ToolCallID: "call_binary",
		ToolName: "web_fetch_exa", Status: "success", OutputJSON: "GIF89a\x00\xff",
	}); err != nil {
		t.Fatalf("create tool call with binary output: %v", err)
	}

	rows, err := repo.ListConversationMessageTraceEventsByMessageIDs(ctx, []uint{1})
	if err != nil {
		t.Fatalf("list trace events: %v", err)
	}
	if len(rows) != 1 || rows[0].Status != "completed" {
		t.Fatalf("expected the completed event to replace the streaming one, got %+v", rows)
	}
	if strings.ContainsRune(rows[0].ContentMarkdown, 0) || !strings.Contains(rows[0].ContentMarkdown, "GIF89a") {
		t.Fatalf("unexpected stored markdown: %q", rows[0].ContentMarkdown)
	}
}
