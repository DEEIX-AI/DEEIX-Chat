package processing

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	appembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/embedding"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/extraction"
	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	memorycache "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/cache/memory"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/objectstorage"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	persistenceconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/postgres/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/sqlitevec"
	portembedding "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/embedding"
	extractionport "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/extraction"
	portobjectstorage "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/objectstorage"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// onePixelPNG 是最小的合法 PNG，供按原图向量化的图片场景读取。
const onePixelPNG = "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89\x00\x00\x00\rIDATx\x9cc\xf8\x0f\x00\x00\x01\x01\x00\x05\x18\xd8N\x00\x00\x00\x00IEND\xaeB`\x82"

// countingExtractor 模拟提取引擎（例如付费 OCR），记录调用次数用于核对重新处理不会重复付费。
type countingExtractor struct {
	text  string
	calls atomic.Int32
}

func (e *countingExtractor) ExtractText(context.Context, extractionport.DocumentRequest) (string, error) {
	e.calls.Add(1)
	return e.text, nil
}

type fixedVectorClient struct{ calls atomic.Int32 }

func (c *fixedVectorClient) CallAPI(_ context.Context, input portembedding.Request) ([][]float32, error) {
	c.calls.Add(1)
	vectors := make([][]float32, len(input.Inputs))
	for i := range vectors {
		vectors[i] = []float32{1, 0, 0}
	}
	return vectors, nil
}

type localStoreProvider struct{ store portobjectstorage.Store }

func (p localStoreProvider) Open(context.Context) (portobjectstorage.Store, error) {
	return p.store, nil
}

type reprocessFixture struct {
	db           *gorm.DB
	extractor    *countingExtractor
	vectors      *fixedVectorClient
	queue        *memorycache.Cache
	service      *Service
	embeddingSvc *appembedding.Service
}

// newReprocessFixture 用真实仓储（SQLite）、真实提取与向量化服务组装处理流水线，
// 只替换外部引擎，用来核对重新处理的完整链路与提取调用次数。
func newReprocessFixture(t *testing.T, dbName string, extractedText string) reprocessFixture {
	return newReprocessFixtureWith(t, dbName, extractedText, reprocessSeed{
		fileName: "scan.pdf", mimeType: "application/pdf", category: "pdf",
		processingStatus: "ready", extractStatus: domainconversation.FileSubprocessStatusEmpty,
		embedStatus: domainconversation.FileSubprocessStatusEmpty,
	}, nil)
}

type reprocessSeed struct {
	fileName         string
	mimeType         string
	category         string
	processingStatus string
	extractStatus    string
	embedStatus      string
}

func newReprocessFixtureWith(t *testing.T, dbName string, extractedText string, seed reprocessSeed, tune func(*config.Config)) reprocessFixture {
	t.Helper()
	ctx := context.Background()
	sqlitevec.Register()
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(&models.FileObject{}, &models.FileChunk{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := sqlitevec.Migrate(db); err != nil {
		t.Fatalf("migrate vectors: %v", err)
	}
	repo := persistenceconversation.NewRepo(db)

	store := objectstorage.NewLocal(t.TempDir())
	if _, err := store.Put(ctx, "uploads/"+seed.fileName, strings.NewReader(onePixelPNG), portobjectstorage.PutOptions{}); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	// 默认场景：上传时提取结果为空（例如旧 OCR 引擎认不出），自动向量化也已关闭。
	file := models.FileObject{
		FileID: "file_scan", UserID: 7, FileName: seed.fileName, MimeType: seed.mimeType, DetectedMIME: seed.mimeType,
		FileCategory: seed.category, StoragePath: "uploads/" + seed.fileName, Status: "active",
		ProcessingStatus: seed.processingStatus, ExtractStatus: seed.extractStatus, EmbedStatus: seed.embedStatus,
	}
	if err := db.Create(&file).Error; err != nil {
		t.Fatalf("seed row: %v", err)
	}

	cfg := config.Config{
		ExtractEngine:             extraction.EngineTika,
		EmbeddingEnabled:          true,
		EmbedTriggerOnUpload:      false,
		RAGModel:                  "text-embedding-test",
		EmbeddingHost:             "https://embedding.example/v1",
		EmbedChunkSizeTokens:      200,
		EmbeddingOutputDimensions: 3,
	}
	if tune != nil {
		tune(&cfg)
	}
	runtime := config.NewRuntime(cfg)
	extractor := &countingExtractor{text: extractedText}
	extractSvc := extraction.NewServiceWithRuntime(runtime, extraction.EngineFactories{
		NewTika: func(config.Config) extraction.DocumentExtractor { return extractor },
	})
	extractSvc.SetObjectStoreProvider(localStoreProvider{store: store})
	vectors := &fixedVectorClient{}
	embeddingSvc := appembedding.NewServiceWithRuntime(runtime, repo, extractSvc, vectors, nil)
	queue := memorycache.New()
	service := NewServiceWithRuntime(Dependencies{
		Config:           runtime,
		Repository:       repo,
		Cache:            queue,
		ExtractService:   extractSvc,
		EmbeddingService: embeddingSvc,
		ExtractorVersion: DefaultExtractorVersion,
	})
	return reprocessFixture{db: db, extractor: extractor, vectors: vectors, queue: queue, service: service, embeddingSvc: embeddingSvc}
}

// runReprocessing 以管理员身份重试文件并执行它进入的文件处理队列消息。
func (f reprocessFixture) runReprocessing(t *testing.T) models.FileObject {
	t.Helper()
	ctx := context.Background()
	result, err := f.service.SubmitAdminFileEmbeddings(ctx, []string{"file_scan"})
	if err != nil || len(result.SubmittedFileIDs) != 1 {
		t.Fatalf("submit retry: result=%+v err=%v", result, err)
	}
	// 重新处理任务进入文件处理队列，而不是向量化队列。
	messages, err := f.queue.ReadFileProcessingMessages(ctx, "worker")
	if err != nil || len(messages) != 1 || messages[0].FileID != "file_scan" {
		t.Fatalf("processing queue = %+v err=%v", messages, err)
	}
	f.service.handleProcessingMessage(ctx, "worker", messages[0])
	var stored models.FileObject
	if err := f.db.Where("file_id = ?", "file_scan").Take(&stored).Error; err != nil {
		t.Fatalf("reload file: %v", err)
	}
	return stored
}

func TestReprocessingNoTextFileSavesTextAndBuildsIndex(t *testing.T) {
	ctx := context.Background()
	fixture := newReprocessFixture(t, "reprocess_success", "这是更换 OCR 引擎后识别出的合同正文。")
	db, extractor, vectors, embeddingSvc := fixture.db, fixture.extractor, fixture.vectors, fixture.embeddingSvc

	stored := fixture.runReprocessing(t)
	// 提取结果写回文件：用户侧不再显示“无文本”，全文注入也能读到这段文本。
	if stored.ExtractStatus != "ready" || !stored.ProcessingReady || stored.ExtractStoragePath == "" || stored.ExtractChars == 0 {
		t.Fatalf("extraction must be persisted: %+v", stored)
	}
	// 关闭了上传后自动向量化，显式重试仍要建立索引。
	if stored.EmbedStatus != "ready" || stored.ChunkCount == 0 {
		t.Fatalf("index must be built after reprocessing: embed=%s chunks=%d err=%q", stored.EmbedStatus, stored.ChunkCount, stored.EmbedError)
	}
	if extractor.calls.Load() != 1 || vectors.calls.Load() == 0 {
		t.Fatalf("expected one extraction and at least one embedding call, got extract=%d embed=%d", extractor.calls.Load(), vectors.calls.Load())
	}

	// 之后再补建（例如更换向量模型）只读取已保存的文本，不会再次付费提取。
	if err := db.Model(&models.FileObject{}).Where("file_id = ?", "file_scan").Update("embed_status", "stale").Error; err != nil {
		t.Fatalf("mark stale: %v", err)
	}
	if err := embeddingSvc.ProcessFile(ctx, toDomainFile(t, db, "file_scan")); err != nil {
		t.Fatalf("rebuild from saved text: %v", err)
	}
	if extractor.calls.Load() != 1 {
		t.Fatalf("rebuilding the index must reuse the saved text, extraction ran %d times", extractor.calls.Load())
	}
}

func toDomainFile(t *testing.T, db *gorm.DB, fileID string) domainconversation.FileObject {
	t.Helper()
	repo := persistenceconversation.NewRepo(db)
	var row models.FileObject
	if err := db.Where("file_id = ?", fileID).Take(&row).Error; err != nil {
		t.Fatalf("load %s: %v", fileID, err)
	}
	file, err := repo.GetActiveFileObjectByID(context.Background(), row.UserID, fileID)
	if err != nil || file == nil {
		t.Fatalf("load domain file: %v", err)
	}
	return *file
}

func TestReprocessingFailureDoesNotLeaveIndexQueued(t *testing.T) {
	// 新引擎仍然提取失败：文件落为处理失败且可再次重试，向量化请求不能一直停在排队中。
	stored := newReprocessFixture(t, "reprocess_failed", "   ").runReprocessing(t)
	if stored.ProcessingStatus != "failed" {
		t.Fatalf("pipeline must settle as failed, got processing_status=%s", stored.ProcessingStatus)
	}
	if stored.EmbedStatus != "none" {
		t.Fatalf("the queued embedding request must be released, got embed_status=%s", stored.EmbedStatus)
	}
}

func TestReprocessingImageWithoutOCREmbedsPixels(t *testing.T) {
	// OCR 曾开启时这张图片识别失败；现在关闭了 OCR 并改用支持图片输入的协议，重试应按原图建立索引，
	// 不能让登记的向量化请求停在排队中。
	fixture := newReprocessFixtureWith(t, "reprocess_image", "", reprocessSeed{
		fileName: "photo.png", mimeType: "image/png", category: "image",
		processingStatus: "failed", extractStatus: "failed", embedStatus: "none",
	}, func(cfg *config.Config) {
		cfg.ExtractImageOCREnabled = false
		cfg.EmbeddingProtocol = config.EmbeddingProtocolGemini
	})
	stored := fixture.runReprocessing(t)
	if stored.ProcessingStatus != "ready" || stored.EmbedStatus != "ready" || stored.ChunkCount != 1 {
		t.Fatalf("image must be indexed from pixels: processing=%s embed=%s chunks=%d err=%q",
			stored.ProcessingStatus, stored.EmbedStatus, stored.ChunkCount, stored.EmbedError)
	}
	if fixture.extractor.calls.Load() != 0 {
		t.Fatalf("pixel embedding must not run text extraction, got %d calls", fixture.extractor.calls.Load())
	}
}
