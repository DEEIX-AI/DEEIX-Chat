package channel

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	appstorage "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/objectstorage"
	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/objectstore"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

func TestUploadModelIconAssetStoresAndDeduplicatesValidatedImage(t *testing.T) {
	repo := newModelIconAssetRepoFake()
	store := objectstore.NewLocal(t.TempDir())
	service := &Service{iconAssetRepo: repo, objectStoreProvider: modelIconStoreProvider{store: store}}
	data := encodeTestModelIconPNG(t, 4, 3)

	first, err := service.UploadModelIconAsset(t.Context(), 7, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("upload first icon: %v", err)
	}
	if first.Reused || first.Ref != ModelIconAssetRefPrefix+first.PublicID || first.ContentType != "image/png" {
		t.Fatalf("unexpected first upload: %#v", first)
	}
	if first.Width != 4 || first.Height != 3 || first.SizeBytes != int64(len(data)) {
		t.Fatalf("unexpected icon metadata: %#v", first)
	}
	stored, err := repo.GetModelIconAssetByPublicID(t.Context(), first.PublicID)
	if err != nil {
		t.Fatalf("load stored icon metadata: %v", err)
	}
	corrupted := append([]byte(nil), data...)
	corrupted[len(corrupted)-1] ^= 0xff
	if _, err = store.Put(t.Context(), stored.StoragePath, bytes.NewReader(corrupted), objectstore.PutOptions{
		SizeBytes: int64(len(corrupted)), ContentType: "image/png",
	}); err != nil {
		t.Fatalf("corrupt stored icon to exercise integrity repair: %v", err)
	}

	second, err := service.UploadModelIconAsset(t.Context(), 8, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("upload duplicate icon: %v", err)
	}
	if !second.Reused || second.Ref != first.Ref || repo.count() != 1 {
		t.Fatalf("duplicate was not reused: first=%#v second=%#v count=%d", first, second, repo.count())
	}
	if err = store.Delete(t.Context(), stored.StoragePath); err != nil {
		t.Fatalf("remove stored icon to exercise missing-object repair: %v", err)
	}
	third, err := service.UploadModelIconAsset(t.Context(), 9, bytes.NewReader(data))
	if err != nil || !third.Reused || third.Ref != first.Ref {
		t.Fatalf("missing object was not repaired: result=%#v error=%v", third, err)
	}

	info, err := service.GetModelIconAssetInfo(t.Context(), first.PublicID)
	if err != nil {
		t.Fatalf("load icon metadata: %v", err)
	}
	content, err := service.OpenModelIconAsset(t.Context(), *info)
	if err != nil {
		t.Fatalf("open icon: %v", err)
	}
	defer content.Reader.Close()
	got, err := io.ReadAll(content.Reader)
	if err != nil {
		t.Fatalf("read icon: %v", err)
	}
	if !bytes.Equal(got, data) || content.ContentType != "image/png" {
		t.Fatalf("opened icon mismatch")
	}
}

func TestUploadModelIconAssetRejectsOversizedAndUnsupportedContent(t *testing.T) {
	service := &Service{
		iconAssetRepo:       newModelIconAssetRepoFake(),
		objectStoreProvider: modelIconStoreProvider{store: objectstore.NewLocal(t.TempDir())},
	}

	_, err := service.UploadModelIconAsset(t.Context(), 1, bytes.NewReader(make([]byte, MaxModelIconBytes+1)))
	if !errors.Is(err, ErrModelIconFileTooLarge) {
		t.Fatalf("oversized upload error = %v", err)
	}
	_, err = service.UploadModelIconAsset(t.Context(), 1, bytes.NewReader([]byte("not an image")))
	if !errors.Is(err, ErrInvalidModelIconFile) {
		t.Fatalf("invalid upload error = %v", err)
	}
	valid := encodeTestModelIconPNG(t, 4, 3)
	_, err = service.UploadModelIconAsset(t.Context(), 1, bytes.NewReader(valid[:33]))
	if !errors.Is(err, ErrInvalidModelIconFile) {
		t.Fatalf("truncated upload error = %v", err)
	}
	_, err = service.UploadModelIconAsset(t.Context(), 1, bytes.NewReader(encodeTestModelIconPNG(t, maxModelIconDimension+1, 1)))
	if !errors.Is(err, ErrInvalidModelIconFile) {
		t.Fatalf("oversized dimensions error = %v", err)
	}
}

func TestReserveModelIconReferenceRequiresReadyManagedAsset(t *testing.T) {
	repo := newModelIconAssetRepoFake()
	service := &Service{iconAssetRepo: repo}

	if err := service.reserveModelIconReference(t.Context(), "asset:ico_00000000000000000000000000000000"); !errors.Is(err, ErrModelIconAssetNotFound) {
		t.Fatalf("missing asset error = %v", err)
	}
	if err := service.reserveModelIconReference(t.Context(), "openai"); err != nil {
		t.Fatalf("built-in slug should remain valid: %v", err)
	}
	readyAt := time.Now()
	item := domainchannel.ModelIconAsset{
		PublicID: "ico_00000000000000000000000000000003", SHA256: strings.Repeat("c", 64),
		StoragePath: "model-icons/c.png", ContentType: "image/png", SizeBytes: 1, Width: 1, Height: 1,
		ReadyAt: &readyAt, LeaseExpiresAt: time.Now().Add(time.Minute),
	}
	if err := repo.CreateModelIconAsset(t.Context(), &item); err != nil {
		t.Fatalf("create managed asset: %v", err)
	}
	if err := service.reserveModelIconReference(t.Context(), ModelIconAssetRefPrefix+item.PublicID); err != nil {
		t.Fatalf("reserve managed asset: %v", err)
	}
	reserved, err := repo.GetModelIconAssetByPublicID(t.Context(), item.PublicID)
	if err != nil || !reserved.LeaseExpiresAt.After(time.Now().Add(modelIconLeaseTTL-time.Minute)) {
		t.Fatalf("managed asset lease was not extended: item=%#v error=%v", reserved, err)
	}
}

func TestCleanupExpiredModelIconAssetsKeepsReferencedAssets(t *testing.T) {
	repo := newModelIconAssetRepoFake()
	store := objectstore.NewLocal(t.TempDir())
	service := &Service{iconAssetRepo: repo, objectStoreProvider: modelIconStoreProvider{store: store}}

	expiredAt := time.Now().Add(-time.Hour)
	temporary := seedTestModelIconAsset(t, repo, store, "ico_00000000000000000000000000000001", expiredAt)
	retained := seedTestModelIconAsset(t, repo, store, "ico_00000000000000000000000000000002", expiredAt)
	repo.setReferenced(ModelIconAssetRefPrefix+retained.PublicID, true)

	if err := service.cleanupExpiredModelIconAssets(t.Context(), store, time.Now()); err != nil {
		t.Fatalf("cleanup expired assets: %v", err)
	}
	if _, err := repo.GetModelIconAssetByPublicID(t.Context(), temporary.PublicID); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expired temporary asset was not removed: %v", err)
	}
	if reader, _, err := store.Open(t.Context(), temporary.StoragePath); !errors.Is(err, objectstore.ErrNotFound) {
		if reader != nil {
			_ = reader.Close()
		}
		t.Fatalf("expired temporary object was not removed: %v", err)
	}
	if _, err := repo.GetModelIconAssetByPublicID(t.Context(), retained.PublicID); err != nil {
		t.Fatalf("retained asset was removed: %v", err)
	}
}

func TestCleanupModelIconAssetStartsFullUnreferencedGracePeriod(t *testing.T) {
	repo := newModelIconAssetRepoFake()
	store := objectstore.NewLocal(t.TempDir())
	service := &Service{iconAssetRepo: repo, objectStoreProvider: modelIconStoreProvider{store: store}}
	item := seedTestModelIconAsset(t, repo, store, "ico_00000000000000000000000000000006", time.Now().Add(-time.Hour))
	repo.clearUnreferenced(item.PublicID)

	now := time.Now()
	if err := service.cleanupExpiredModelIconAssets(t.Context(), store, now); err != nil {
		t.Fatalf("start unreferenced grace period: %v", err)
	}
	pending, err := repo.GetModelIconAssetByPublicID(t.Context(), item.PublicID)
	if err != nil || pending.UnreferencedAt == nil || pending.LeaseExpiresAt.Before(now.Add(modelIconLeaseTTL-time.Minute)) {
		t.Fatalf("asset did not receive a full grace period: item=%#v error=%v", pending, err)
	}

	repo.expire(item.PublicID, time.Now().Add(-time.Minute))
	if err = service.cleanupExpiredModelIconAssets(t.Context(), store, time.Now()); err != nil {
		t.Fatalf("cleanup after grace period: %v", err)
	}
	if _, err = repo.GetModelIconAssetByPublicID(t.Context(), item.PublicID); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("asset survived completed grace period: %v", err)
	}
}

func TestModelIconAssetLibraryRemovalRejectsReferencesAndHidesUnusedAsset(t *testing.T) {
	repo := newModelIconAssetRepoFake()
	store := objectstore.NewLocal(t.TempDir())
	service := &Service{iconAssetRepo: repo, objectStoreProvider: modelIconStoreProvider{store: store}}
	uploaded, err := service.UploadModelIconAsset(t.Context(), 1, bytes.NewReader(encodeTestModelIconPNG(t, 2, 2)))
	if err != nil {
		t.Fatalf("upload icon: %v", err)
	}
	items, total, err := service.ListModelIconAssets(t.Context(), 1, 20)
	if err != nil || total != 1 || len(items) != 1 || items[0].Ref != uploaded.Ref {
		t.Fatalf("unexpected asset library: total=%d items=%#v error=%v", total, items, err)
	}

	repo.setReferenced(uploaded.Ref, true)
	err = service.RequestModelIconAssetDeletion(t.Context(), uploaded.PublicID)
	var inUse *ModelIconAssetInUseError
	if !errors.As(err, &inUse) || inUse.References.Total() != 1 {
		t.Fatalf("referenced deletion error = %#v", err)
	}

	repo.setReferenced(uploaded.Ref, false)
	if err = service.RequestModelIconAssetDeletion(t.Context(), uploaded.PublicID); err != nil {
		t.Fatalf("remove unused icon from library: %v", err)
	}
	items, total, err = service.ListModelIconAssets(t.Context(), 1, 20)
	if err != nil || total != 0 || len(items) != 0 {
		t.Fatalf("removed icon remains in library: total=%d items=%#v error=%v", total, items, err)
	}
	if _, err = service.GetModelIconAssetInfo(t.Context(), uploaded.PublicID); err != nil {
		t.Fatalf("icon must remain readable during safety window: %v", err)
	}
}

func TestUploadModelIconAssetRenewsExpiredDuplicateBeforeCleanup(t *testing.T) {
	repo := newModelIconAssetRepoFake()
	store := objectstore.NewLocal(t.TempDir())
	service := &Service{iconAssetRepo: repo, objectStoreProvider: modelIconStoreProvider{store: store}}
	data := encodeTestModelIconPNG(t, 2, 2)
	first, err := service.UploadModelIconAsset(t.Context(), 1, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("upload first icon: %v", err)
	}
	repo.expire(first.PublicID, time.Now().Add(-time.Hour))

	reused, err := service.UploadModelIconAsset(t.Context(), 2, bytes.NewReader(data))
	if err != nil || !reused.Reused || reused.PublicID != first.PublicID {
		t.Fatalf("reuse expired icon: result=%#v error=%v", reused, err)
	}
	if err = service.cleanupExpiredModelIconAssets(t.Context(), store, time.Now()); err != nil {
		t.Fatalf("cleanup after lease renewal: %v", err)
	}
	if _, err = repo.GetModelIconAssetByPublicID(t.Context(), first.PublicID); err != nil {
		t.Fatalf("renewed duplicate was deleted: %v", err)
	}
}

func TestCleanupExpiredModelIconAssetRetriesObjectDeletion(t *testing.T) {
	repo := newModelIconAssetRepoFake()
	local := objectstore.NewLocal(t.TempDir())
	store := &modelIconDeleteFailureStore{Store: local, fail: true}
	service := &Service{iconAssetRepo: repo, objectStoreProvider: modelIconStoreProvider{store: store}}
	item := seedTestModelIconAsset(t, repo, local, "ico_00000000000000000000000000000004", time.Now().Add(-time.Hour))

	if err := service.cleanupExpiredModelIconAssets(t.Context(), store, time.Now()); err == nil {
		t.Fatal("expected first object deletion to fail")
	}
	pending, err := repo.GetModelIconAssetByPublicID(t.Context(), item.PublicID)
	if err != nil || pending.DeletingAt == nil {
		t.Fatalf("failed deletion was not retained for retry: item=%#v error=%v", pending, err)
	}
	store.fail = false
	if err = service.cleanupExpiredModelIconAssets(t.Context(), store, time.Now()); err != nil {
		t.Fatalf("retry object deletion: %v", err)
	}
	if _, err = repo.GetModelIconAssetByPublicID(t.Context(), item.PublicID); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("retried asset metadata was not removed: %v", err)
	}
}

func TestUploadModelIconAssetKeepsFailedObjectWriteRecoverable(t *testing.T) {
	repo := newModelIconAssetRepoFake()
	local := objectstore.NewLocal(t.TempDir())
	store := &modelIconPutFailureStore{Store: local, fail: true}
	service := &Service{iconAssetRepo: repo, objectStoreProvider: modelIconStoreProvider{store: store}}

	if _, err := service.UploadModelIconAsset(t.Context(), 1, bytes.NewReader(encodeTestModelIconPNG(t, 2, 2))); err == nil {
		t.Fatal("expected object write to fail")
	}
	if repo.count() != 1 {
		t.Fatalf("pending metadata count = %d, want 1", repo.count())
	}
	var pending domainchannel.ModelIconAsset
	repo.mu.Lock()
	for _, item := range repo.byID {
		pending = item
	}
	repo.mu.Unlock()
	if pending.ReadyAt != nil {
		t.Fatalf("failed upload was marked ready: %#v", pending)
	}
	if _, err := service.GetModelIconAssetInfo(t.Context(), pending.PublicID); !errors.Is(err, ErrModelIconAssetNotFound) {
		t.Fatalf("pending upload public lookup error = %v", err)
	}
	repo.expire(pending.PublicID, time.Now().Add(-time.Hour))
	store.fail = false
	if err := service.cleanupExpiredModelIconAssets(t.Context(), store, time.Now()); err != nil {
		t.Fatalf("cleanup failed upload: %v", err)
	}
	if repo.count() != 0 {
		t.Fatalf("failed upload metadata was not cleaned: count=%d", repo.count())
	}
}

func TestOpenModelIconAssetRejectsMetadataAndObjectMismatches(t *testing.T) {
	repo := newModelIconAssetRepoFake()
	store := objectstore.NewLocal(t.TempDir())
	service := &Service{iconAssetRepo: repo, objectStoreProvider: modelIconStoreProvider{store: store}}
	uploaded, err := service.UploadModelIconAsset(t.Context(), 1, bytes.NewReader(encodeTestModelIconPNG(t, 2, 2)))
	if err != nil {
		t.Fatalf("upload icon: %v", err)
	}
	info, err := service.GetModelIconAssetInfo(t.Context(), uploaded.PublicID)
	if err != nil {
		t.Fatalf("get icon info: %v", err)
	}

	tampered := *info
	tampered.StoragePath = "model-icons/unrelated.png"
	if _, err = service.OpenModelIconAsset(t.Context(), tampered); !errors.Is(err, ErrModelIconAssetUnavailable) {
		t.Fatalf("tampered path error = %v", err)
	}
	if _, err = store.Put(t.Context(), info.StoragePath, bytes.NewReader([]byte("wrong-size")), objectstore.PutOptions{
		SizeBytes: 10, ContentType: info.ContentType,
	}); err != nil {
		t.Fatalf("replace icon object: %v", err)
	}
	if _, err = service.OpenModelIconAsset(t.Context(), *info); !errors.Is(err, ErrModelIconAssetUnavailable) {
		t.Fatalf("mismatched object size error = %v", err)
	}
}

func TestCleanupExpiredModelIconAssetRetriesMetadataDeletion(t *testing.T) {
	repo := newModelIconAssetRepoFake()
	store := objectstore.NewLocal(t.TempDir())
	service := &Service{iconAssetRepo: repo, objectStoreProvider: modelIconStoreProvider{store: store}}
	item := seedTestModelIconAsset(t, repo, store, "ico_00000000000000000000000000000005", time.Now().Add(-time.Hour))
	repo.deleteClaimedErr = errors.New("injected metadata deletion failure")

	if err := service.cleanupExpiredModelIconAssets(t.Context(), store, time.Now()); err == nil {
		t.Fatal("expected first metadata deletion to fail")
	}
	pending, err := repo.GetModelIconAssetByPublicID(t.Context(), item.PublicID)
	if err != nil || pending.DeletingAt == nil {
		t.Fatalf("metadata deletion failure was not retained for retry: item=%#v error=%v", pending, err)
	}
	repo.deleteClaimedErr = nil
	if err = service.cleanupExpiredModelIconAssets(t.Context(), store, time.Now()); err != nil {
		t.Fatalf("retry metadata deletion: %v", err)
	}
	if _, err = repo.GetModelIconAssetByPublicID(t.Context(), item.PublicID); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("retried metadata was not removed: %v", err)
	}
}

func TestNormalizeModelPresentationIconAllowsOnlyDocumentedFormats(t *testing.T) {
	valid := map[string]string{
		" OpenAI ":                     "openai",
		"https://example.com/icon.png": "https://example.com/icon.png",
		"/assets/icon.png":             "/assets/icon.png",
		"ASSET:ICO_00000000000000000000000000000001": "asset:ico_00000000000000000000000000000001",
	}
	for input, expected := range valid {
		actual, err := normalizeModelPresentationIcon(input)
		if err != nil || actual != expected {
			t.Fatalf("normalize valid icon %q = %q, %v", input, actual, err)
		}
	}
	for _, input := range []string{"data:image/png;base64,AAAA", "javascript:alert(1)", "../icon.png", "//example.com/icon.png"} {
		if _, err := normalizeModelPresentationIcon(input); !errors.Is(err, ErrInvalidModelIconReference) {
			t.Fatalf("invalid icon %q error = %v", input, err)
		}
	}
}

func seedTestModelIconAsset(
	t *testing.T,
	repo *modelIconAssetRepoFake,
	store objectstore.Store,
	publicID string,
	leaseExpiresAt time.Time,
) domainchannel.ModelIconAsset {
	t.Helper()
	data := encodeTestModelIconPNG(t, 1, 1)
	hash := publicID[len("ico_"):] + publicID[len("ico_"):]
	readyAt := time.Now()
	item := domainchannel.ModelIconAsset{
		PublicID: publicID, SHA256: hash, StoragePath: "model-icons/test/" + publicID + ".png",
		ContentType: "image/png", SizeBytes: int64(len(data)), Width: 1, Height: 1,
		CreatedByUserID: 1, ReadyAt: &readyAt, LeaseExpiresAt: leaseExpiresAt,
	}
	unreferencedAt := leaseExpiresAt.Add(-modelIconLeaseTTL)
	item.UnreferencedAt = &unreferencedAt
	if _, err := store.Put(t.Context(), item.StoragePath, bytes.NewReader(data), objectstore.PutOptions{SizeBytes: int64(len(data)), ContentType: item.ContentType}); err != nil {
		t.Fatalf("seed icon object: %v", err)
	}
	if err := repo.CreateModelIconAsset(t.Context(), &item); err != nil {
		t.Fatalf("seed icon metadata: %v", err)
	}
	return item
}

func encodeTestModelIconPNG(t *testing.T, width int, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	img.Set(0, 0, color.RGBA{R: 20, G: 40, B: 60, A: 255})
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buffer.Bytes()
}

type modelIconStoreProvider struct {
	store objectstore.Store
}

type modelIconDeleteFailureStore struct {
	objectstore.Store
	fail bool
}

type modelIconPutFailureStore struct {
	objectstore.Store
	fail bool
}

func (s *modelIconPutFailureStore) Put(ctx context.Context, key string, body io.Reader, opts objectstore.PutOptions) (objectstore.ObjectInfo, error) {
	if s.fail {
		return objectstore.ObjectInfo{}, errors.New("injected object write failure")
	}
	return s.Store.Put(ctx, key, body, opts)
}

func (s *modelIconDeleteFailureStore) Delete(ctx context.Context, key string) error {
	if s.fail {
		return errors.New("injected object deletion failure")
	}
	return s.Store.Delete(ctx, key)
}

func (p modelIconStoreProvider) Open(context.Context) (objectstore.Store, error) {
	return p.store, nil
}

type modelIconAssetRepoFake struct {
	mu               sync.Mutex
	byID             map[string]domainchannel.ModelIconAsset
	bySHA256         map[string]string
	references       map[string]bool
	deleteClaimedErr error
}

func newModelIconAssetRepoFake() *modelIconAssetRepoFake {
	return &modelIconAssetRepoFake{
		byID:       make(map[string]domainchannel.ModelIconAsset),
		bySHA256:   make(map[string]string),
		references: make(map[string]bool),
	}
}

func (r *modelIconAssetRepoFake) CreateModelIconAsset(_ context.Context, item *domainchannel.ModelIconAsset) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.bySHA256[item.SHA256]; exists {
		return repository.ErrDuplicate
	}
	item.ID = uint(len(r.byID) + 1)
	if item.CreatedAt.IsZero() {
		item.CreatedAt = time.Now()
	}
	r.byID[item.PublicID] = *item
	r.bySHA256[item.SHA256] = item.PublicID
	return nil
}

func (r *modelIconAssetRepoFake) RefreshModelIconAssetUploadLease(_ context.Context, publicID string, unreferencedAt time.Time, leaseExpiresAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, exists := r.byID[publicID]
	if !exists || item.DeletingAt != nil {
		return repository.ErrNotFound
	}
	item.LeaseExpiresAt = leaseExpiresAt
	item.UnreferencedAt = &unreferencedAt
	item.DeleteRequestedAt = nil
	r.byID[publicID] = item
	return nil
}

func (r *modelIconAssetRepoFake) MarkModelIconAssetReady(_ context.Context, publicID string, readyAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, exists := r.byID[publicID]
	if !exists || item.DeletingAt != nil {
		return repository.ErrNotFound
	}
	if item.ReadyAt == nil {
		item.ReadyAt = &readyAt
	}
	r.byID[publicID] = item
	return nil
}

func (r *modelIconAssetRepoFake) ReserveModelIconAssetReference(_ context.Context, publicID string, leaseExpiresAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, exists := r.byID[publicID]
	if !exists || item.ReadyAt == nil || item.DeletingAt != nil {
		return repository.ErrNotFound
	}
	item.LeaseExpiresAt = leaseExpiresAt
	item.UnreferencedAt = nil
	item.DeleteRequestedAt = nil
	r.byID[publicID] = item
	return nil
}

func (r *modelIconAssetRepoFake) ListModelIconAssets(_ context.Context, offset int, limit int) ([]domainchannel.ModelIconAsset, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]domainchannel.ModelIconAsset, 0)
	for _, item := range r.byID {
		if item.ReadyAt != nil && item.DeletingAt == nil && item.DeleteRequestedAt == nil {
			items = append(items, item)
		}
	}
	total := int64(len(items))
	if offset >= len(items) || limit <= 0 {
		return []domainchannel.ModelIconAsset{}, total, nil
	}
	end := min(offset+limit, len(items))
	return items[offset:end], total, nil
}

func (r *modelIconAssetRepoFake) ListExpiredModelIconAssets(_ context.Context, expiredBefore time.Time, limit int) ([]domainchannel.ModelIconAsset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]domainchannel.ModelIconAsset, 0)
	for _, item := range r.byID {
		if item.DeletingAt != nil || !item.LeaseExpiresAt.After(expiredBefore) {
			items = append(items, item)
			if len(items) == limit {
				break
			}
		}
	}
	return items, nil
}

func (r *modelIconAssetRepoFake) HasModelIconAssetReference(_ context.Context, ref string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.references[ref], nil
}

func (r *modelIconAssetRepoFake) GetModelIconAssetReferenceSummary(_ context.Context, ref string) (repository.ModelIconAssetReferenceSummary, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.references[ref] {
		return repository.ModelIconAssetReferenceSummary{Models: 1}, nil
	}
	return repository.ModelIconAssetReferenceSummary{}, nil
}

func (r *modelIconAssetRepoFake) MarkModelIconAssetUnreferenced(_ context.Context, assetID uint, expiredBefore time.Time, unreferencedAt time.Time, leaseExpiresAt time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for publicID, item := range r.byID {
		if item.ID != assetID || item.DeletingAt != nil || item.UnreferencedAt != nil || item.LeaseExpiresAt.After(expiredBefore) {
			continue
		}
		item.UnreferencedAt = &unreferencedAt
		item.LeaseExpiresAt = leaseExpiresAt
		r.byID[publicID] = item
		return true, nil
	}
	return false, nil
}

func (r *modelIconAssetRepoFake) RequestModelIconAssetDeletion(_ context.Context, assetID uint, requestedAt time.Time, leaseExpiresAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for publicID, item := range r.byID {
		if item.ID != assetID || item.ReadyAt == nil || item.DeletingAt != nil || item.DeleteRequestedAt != nil {
			continue
		}
		item.DeleteRequestedAt = &requestedAt
		item.UnreferencedAt = &requestedAt
		item.LeaseExpiresAt = leaseExpiresAt
		r.byID[publicID] = item
		return nil
	}
	return repository.ErrNotFound
}

func (r *modelIconAssetRepoFake) ClaimModelIconAssetDeletion(_ context.Context, assetID uint, expiredBefore time.Time, deletingAt time.Time) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for publicID, item := range r.byID {
		if item.ID != assetID || item.DeletingAt != nil || item.UnreferencedAt == nil || item.LeaseExpiresAt.After(expiredBefore) {
			continue
		}
		item.DeletingAt = &deletingAt
		r.byID[publicID] = item
		return true, nil
	}
	return false, nil
}

func (r *modelIconAssetRepoFake) DeleteClaimedModelIconAsset(_ context.Context, assetID uint) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.deleteClaimedErr != nil {
		return r.deleteClaimedErr
	}
	for publicID, item := range r.byID {
		if item.ID != assetID || item.DeletingAt == nil {
			continue
		}
		delete(r.byID, publicID)
		delete(r.bySHA256, item.SHA256)
		return nil
	}
	return repository.ErrNotFound
}

func (r *modelIconAssetRepoFake) GetModelIconAssetByPublicID(_ context.Context, publicID string) (*domainchannel.ModelIconAsset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, exists := r.byID[publicID]
	if !exists {
		return nil, repository.ErrNotFound
	}
	copy := item
	return &copy, nil
}

func (r *modelIconAssetRepoFake) GetModelIconAssetBySHA256(_ context.Context, sha256 string) (*domainchannel.ModelIconAsset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	publicID, exists := r.bySHA256[sha256]
	if !exists {
		return nil, repository.ErrNotFound
	}
	item := r.byID[publicID]
	copy := item
	return &copy, nil
}

func (r *modelIconAssetRepoFake) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.byID)
}

func (r *modelIconAssetRepoFake) setReferenced(ref string, referenced bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.references[ref] = referenced
}

func (r *modelIconAssetRepoFake) expire(publicID string, expiresAt time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item := r.byID[publicID]
	item.LeaseExpiresAt = expiresAt
	r.byID[publicID] = item
}

func (r *modelIconAssetRepoFake) clearUnreferenced(publicID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item := r.byID[publicID]
	item.UnreferencedAt = nil
	r.byID[publicID] = item
}

var _ repository.ModelIconAssetRepository = (*modelIconAssetRepoFake)(nil)
var _ appstorage.Provider = modelIconStoreProvider{}
