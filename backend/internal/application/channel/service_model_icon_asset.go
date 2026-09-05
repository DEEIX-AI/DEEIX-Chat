package channel

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"regexp"
	"strings"
	"time"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/objectstore"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/pagination"
	"github.com/google/uuid"
	"go.uber.org/zap"
	_ "golang.org/x/image/webp"
)

const (
	ModelIconAssetRefPrefix = "asset:"
	MaxModelIconBytes       = int64(1 << 20)
	maxModelIconDimension   = 2048
	modelIconCleanupBatch   = 128
	modelIconCleanupPeriod  = time.Hour
	modelIconLeaseTTL       = 24 * time.Hour
)

var (
	modelIconAssetPublicIDPattern = regexp.MustCompile(`^ico_[a-f0-9]{32}$`)
	modelIconAssetSHA256Pattern   = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

// ModelIconAssetUpload 表示管理员上传后的稳定图标引用。
type ModelIconAssetUpload struct {
	Ref         string
	PublicID    string
	ContentType string
	SizeBytes   int64
	Width       int
	Height      int
	Reused      bool
}

// ModelIconAssetContent 表示公开读取的安全图片内容。
type ModelIconAssetContent struct {
	Reader      io.ReadCloser
	ContentType string
	SizeBytes   int64
}

// ModelIconAssetInfo 表示无需打开对象内容即可读取的公开图标元数据。
type ModelIconAssetInfo struct {
	PublicID    string
	StoragePath string
	ContentType string
	SizeBytes   int64
	SHA256      string
}

// ModelIconAssetView 表示管理员图标库中的已上传资产。
type ModelIconAssetView struct {
	Ref         string
	PublicID    string
	ContentType string
	SizeBytes   int64
	Width       int
	Height      int
	CreatedAt   string
}

// ModelIconAssetInUseError 携带阻止管理员移除图标的引用统计。
type ModelIconAssetInUseError struct {
	References repository.ModelIconAssetReferenceSummary
}

func (e *ModelIconAssetInUseError) Error() string { return ErrModelIconAssetInUse.Error() }
func (e *ModelIconAssetInUseError) Unwrap() error { return ErrModelIconAssetInUse }

// StartModelIconAssetCleanup 定期回收租约过期且不再被配置或会话快照引用的图标资产。
func (s *Service) StartModelIconAssetCleanup(ctx context.Context) {
	if ctx == nil || s.objectStoreProvider == nil || s.iconAssetRepo == nil {
		return
	}
	go func() {
		s.runModelIconAssetCleanup(ctx)
		ticker := time.NewTicker(modelIconCleanupPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.runModelIconAssetCleanup(ctx)
			}
		}
	}()
}

func (s *Service) runModelIconAssetCleanup(ctx context.Context) {
	store, err := s.objectStoreProvider.Open(ctx)
	if err == nil {
		err = s.cleanupExpiredModelIconAssets(ctx, store, time.Now())
	}
	if err != nil && s.logger != nil && ctx.Err() == nil {
		s.logger.Warn("model icon asset cleanup failed", zap.Error(err))
	}
}

// UploadModelIconAsset 校验图片、按内容去重并写入统一对象存储。
func (s *Service) UploadModelIconAsset(ctx context.Context, actorUserID uint, reader io.Reader) (*ModelIconAssetUpload, error) {
	if reader == nil {
		return nil, ErrInvalidModelIconFile
	}
	if s.objectStoreProvider == nil || s.iconAssetRepo == nil {
		return nil, ErrModelIconAssetUnavailable
	}

	data, err := io.ReadAll(io.LimitReader(reader, MaxModelIconBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read model icon: %w", err)
	}
	if len(data) == 0 {
		return nil, ErrInvalidModelIconFile
	}
	if int64(len(data)) > MaxModelIconBytes {
		return nil, ErrModelIconFileTooLarge
	}

	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, ErrInvalidModelIconFile
	}
	contentType, extension, ok := allowedModelIconFormat(format)
	if !ok || config.Width <= 0 || config.Height <= 0 ||
		config.Width > maxModelIconDimension || config.Height > maxModelIconDimension {
		return nil, ErrInvalidModelIconFile
	}
	decoded, decodedFormat, err := image.Decode(bytes.NewReader(data))
	if err != nil || !strings.EqualFold(decodedFormat, format) ||
		decoded.Bounds().Dx() != config.Width || decoded.Bounds().Dy() != config.Height {
		return nil, ErrInvalidModelIconFile
	}

	digest := sha256.Sum256(data)
	hash := hex.EncodeToString(digest[:])
	now := time.Now()
	leaseExpiresAt := now.Add(modelIconLeaseTTL)
	storagePath := modelIconStoragePath(hash, extension)
	store, err := s.objectStoreProvider.Open(ctx)
	if err != nil {
		return nil, fmt.Errorf("open model icon store: %w", err)
	}
	if existing, findErr := s.iconAssetRepo.GetModelIconAssetBySHA256(ctx, hash); findErr == nil {
		return s.prepareModelIconAssetUpload(ctx, store, existing, data, leaseExpiresAt, true)
	} else if !errors.Is(findErr, repository.ErrNotFound) {
		return nil, findErr
	}

	item := &domainchannel.ModelIconAsset{
		PublicID: "ico_" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		SHA256:   hash, StoragePath: storagePath, ContentType: contentType,
		SizeBytes: int64(len(data)), Width: config.Width, Height: config.Height,
		CreatedByUserID: actorUserID, LeaseExpiresAt: leaseExpiresAt, UnreferencedAt: &now,
	}
	if err = s.iconAssetRepo.CreateModelIconAsset(ctx, item); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			existing, findErr := s.iconAssetRepo.GetModelIconAssetBySHA256(ctx, hash)
			if findErr == nil {
				return s.prepareModelIconAssetUpload(ctx, store, existing, data, leaseExpiresAt, true)
			}
		}
		return nil, err
	}
	return s.prepareModelIconAssetUpload(ctx, store, item, data, leaseExpiresAt, false)
}

func (s *Service) prepareModelIconAssetUpload(
	ctx context.Context,
	store objectstore.Store,
	item *domainchannel.ModelIconAsset,
	data []byte,
	leaseExpiresAt time.Time,
	reused bool,
) (*ModelIconAssetUpload, error) {
	if item == nil {
		return nil, ErrModelIconAssetUnavailable
	}
	unreferencedAt := leaseExpiresAt.Add(-modelIconLeaseTTL)
	if err := s.iconAssetRepo.RefreshModelIconAssetUploadLease(ctx, item.PublicID, unreferencedAt, leaseExpiresAt); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrModelIconAssetUnavailable
		}
		return nil, err
	}
	if err := ensureModelIconObject(ctx, store, item, data); err != nil {
		return nil, err
	}
	readyAt := time.Now()
	if err := s.iconAssetRepo.MarkModelIconAssetReady(ctx, item.PublicID, readyAt); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrModelIconAssetUnavailable
		}
		return nil, err
	}
	item.LeaseExpiresAt = leaseExpiresAt
	item.UnreferencedAt = &unreferencedAt
	item.DeleteRequestedAt = nil
	if item.ReadyAt == nil {
		item.ReadyAt = &readyAt
	}
	return modelIconAssetUpload(item, reused), nil
}

// ListModelIconAssets 分页查询管理员上传且尚未移出图标库的资产。
func (s *Service) ListModelIconAssets(ctx context.Context, page int, pageSize int) ([]ModelIconAssetView, int64, error) {
	if s.iconAssetRepo == nil {
		return nil, 0, ErrModelIconAssetUnavailable
	}
	offset, limit := pagination.Offset(page, pageSize)
	items, total, err := s.iconAssetRepo.ListModelIconAssets(ctx, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	views := make([]ModelIconAssetView, 0, len(items))
	for _, item := range items {
		views = append(views, ModelIconAssetView{
			Ref: ModelIconAssetRefPrefix + item.PublicID, PublicID: item.PublicID,
			ContentType: item.ContentType, SizeBytes: item.SizeBytes,
			Width: item.Width, Height: item.Height, CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	return views, total, nil
}

// RequestModelIconAssetDeletion 将无引用图标移出资产库，并保留 24 小时可恢复窗口。
func (s *Service) RequestModelIconAssetDeletion(ctx context.Context, publicID string) error {
	normalizedID := strings.ToLower(strings.TrimSpace(publicID))
	if !modelIconAssetPublicIDPattern.MatchString(normalizedID) {
		return ErrModelIconAssetNotFound
	}
	if s.iconAssetRepo == nil {
		return ErrModelIconAssetUnavailable
	}
	item, err := s.iconAssetRepo.GetModelIconAssetByPublicID(ctx, normalizedID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrModelIconAssetNotFound
		}
		return err
	}
	if item.ReadyAt == nil || item.DeletingAt != nil || item.DeleteRequestedAt != nil {
		return ErrModelIconAssetNotFound
	}
	references, err := s.iconAssetRepo.GetModelIconAssetReferenceSummary(ctx, ModelIconAssetRefPrefix+normalizedID)
	if err != nil {
		return err
	}
	if references.Total() > 0 {
		return &ModelIconAssetInUseError{References: references}
	}
	now := time.Now()
	if err = s.iconAssetRepo.RequestModelIconAssetDeletion(ctx, item.ID, now, now.Add(modelIconLeaseTTL)); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrModelIconAssetNotFound
		}
		return err
	}
	return nil
}

// GetModelIconAssetInfo 获取公开图标元数据，不访问对象内容。
func (s *Service) GetModelIconAssetInfo(ctx context.Context, publicID string) (*ModelIconAssetInfo, error) {
	normalizedID := strings.ToLower(strings.TrimSpace(publicID))
	if !modelIconAssetPublicIDPattern.MatchString(normalizedID) {
		return nil, ErrModelIconAssetNotFound
	}
	if s.iconAssetRepo == nil {
		return nil, ErrModelIconAssetUnavailable
	}
	item, err := s.iconAssetRepo.GetModelIconAssetByPublicID(ctx, normalizedID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrModelIconAssetNotFound
		}
		return nil, err
	}
	expectedStoragePath, ok := expectedModelIconStoragePath(item.SHA256, item.ContentType)
	if item.ReadyAt == nil || item.DeletingAt != nil || !ok || item.StoragePath != expectedStoragePath ||
		item.SizeBytes <= 0 || item.SizeBytes > MaxModelIconBytes {
		return nil, ErrModelIconAssetNotFound
	}
	return &ModelIconAssetInfo{
		PublicID: item.PublicID, StoragePath: item.StoragePath, ContentType: item.ContentType,
		SizeBytes: item.SizeBytes, SHA256: item.SHA256,
	}, nil
}

// OpenModelIconAsset 打开已经完成元数据校验的公开图标内容。
func (s *Service) OpenModelIconAsset(ctx context.Context, info ModelIconAssetInfo) (*ModelIconAssetContent, error) {
	expectedStoragePath, valid := expectedModelIconStoragePath(info.SHA256, info.ContentType)
	if s.objectStoreProvider == nil || !modelIconAssetPublicIDPattern.MatchString(info.PublicID) || !valid ||
		info.StoragePath != expectedStoragePath || info.SizeBytes <= 0 || info.SizeBytes > MaxModelIconBytes {
		return nil, ErrModelIconAssetUnavailable
	}
	store, err := s.objectStoreProvider.Open(ctx)
	if err != nil {
		return nil, fmt.Errorf("open model icon store: %w", err)
	}
	reader, objectInfo, err := store.Open(ctx, info.StoragePath)
	if err != nil {
		if errors.Is(err, objectstore.ErrNotFound) {
			return nil, ErrModelIconAssetNotFound
		}
		return nil, err
	}
	if objectInfo.SizeBytes > 0 && objectInfo.SizeBytes != info.SizeBytes {
		_ = reader.Close()
		return nil, ErrModelIconAssetUnavailable
	}
	return &ModelIconAssetContent{Reader: reader, ContentType: info.ContentType, SizeBytes: info.SizeBytes}, nil
}

func modelIconAssetUpload(item *domainchannel.ModelIconAsset, reused bool) *ModelIconAssetUpload {
	return &ModelIconAssetUpload{
		Ref: ModelIconAssetRefPrefix + item.PublicID, PublicID: item.PublicID,
		ContentType: item.ContentType, SizeBytes: item.SizeBytes,
		Width: item.Width, Height: item.Height, Reused: reused,
	}
}

func allowedModelIconFormat(format string) (contentType string, extension string, ok bool) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "png":
		return "image/png", ".png", true
	case "jpeg":
		return "image/jpeg", ".jpg", true
	case "webp":
		return "image/webp", ".webp", true
	default:
		return "", "", false
	}
}

func expectedModelIconStoragePath(hash string, contentType string) (string, bool) {
	if !modelIconAssetSHA256Pattern.MatchString(hash) {
		return "", false
	}
	var extension string
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/png":
		extension = ".png"
	case "image/jpeg":
		extension = ".jpg"
	case "image/webp":
		extension = ".webp"
	default:
		return "", false
	}
	return modelIconStoragePath(hash, extension), true
}

func modelIconStoragePath(hash string, extension string) string {
	return fmt.Sprintf("model-icons/%s/%s%s", hash[:2], hash, extension)
}

func ensureModelIconObject(ctx context.Context, store objectstore.Store, item *domainchannel.ModelIconAsset, data []byte) error {
	reader, _, err := store.Open(ctx, item.StoragePath)
	if err == nil {
		stored, readErr := io.ReadAll(io.LimitReader(reader, MaxModelIconBytes+1))
		closeErr := reader.Close()
		if readErr == nil && closeErr == nil && bytes.Equal(stored, data) {
			return nil
		}
	} else if !errors.Is(err, objectstore.ErrNotFound) {
		return fmt.Errorf("open existing model icon: %w", err)
	}
	if _, err = store.Put(ctx, item.StoragePath, bytes.NewReader(data), objectstore.PutOptions{
		SizeBytes: int64(len(data)), ContentType: item.ContentType,
	}); err != nil {
		return fmt.Errorf("restore model icon: %w", err)
	}
	return nil
}

func modelIconAssetPublicID(ref string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(ref))
	if !strings.HasPrefix(normalized, ModelIconAssetRefPrefix) {
		return "", false
	}
	publicID := strings.TrimPrefix(normalized, ModelIconAssetRefPrefix)
	return publicID, modelIconAssetPublicIDPattern.MatchString(publicID)
}

func (s *Service) reserveModelIconReference(ctx context.Context, icon string) error {
	normalized := strings.TrimSpace(icon)
	if normalized == "" {
		return nil
	}
	if !strings.HasPrefix(strings.ToLower(normalized), ModelIconAssetRefPrefix) {
		return nil
	}
	publicID, ok := modelIconAssetPublicID(normalized)
	if !ok {
		return ErrInvalidModelIconReference
	}
	if s.iconAssetRepo == nil {
		return ErrModelIconAssetUnavailable
	}
	if err := s.iconAssetRepo.ReserveModelIconAssetReference(ctx, publicID, time.Now().Add(modelIconLeaseTTL)); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrModelIconAssetNotFound
		}
		return err
	}
	return nil
}

func (s *Service) cleanupExpiredModelIconAssets(ctx context.Context, store objectstore.Store, expiredBefore time.Time) error {
	items, err := s.iconAssetRepo.ListExpiredModelIconAssets(ctx, expiredBefore, modelIconCleanupBatch)
	if err != nil {
		return err
	}
	var cleanupErr error
	for _, item := range items {
		if item.DeletingAt == nil {
			ref := ModelIconAssetRefPrefix + item.PublicID
			referenced, referenceErr := s.iconAssetRepo.HasModelIconAssetReference(ctx, ref)
			if referenceErr != nil {
				cleanupErr = errors.Join(cleanupErr, referenceErr)
				continue
			}
			if referenced {
				if reserveErr := s.iconAssetRepo.ReserveModelIconAssetReference(ctx, item.PublicID, expiredBefore.Add(modelIconLeaseTTL)); reserveErr != nil {
					cleanupErr = errors.Join(cleanupErr, reserveErr)
				}
				continue
			}
			if item.UnreferencedAt == nil {
				_, markErr := s.iconAssetRepo.MarkModelIconAssetUnreferenced(
					ctx, item.ID, expiredBefore, expiredBefore, expiredBefore.Add(modelIconLeaseTTL),
				)
				if markErr != nil {
					cleanupErr = errors.Join(cleanupErr, markErr)
				}
				continue
			}
			claimed, claimErr := s.iconAssetRepo.ClaimModelIconAssetDeletion(ctx, item.ID, expiredBefore, time.Now())
			if claimErr != nil {
				cleanupErr = errors.Join(cleanupErr, claimErr)
				continue
			}
			if !claimed {
				continue
			}
		}
		if deleteErr := store.Delete(ctx, item.StoragePath); deleteErr != nil && !errors.Is(deleteErr, objectstore.ErrNotFound) {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("delete model icon object %s: %w", item.PublicID, deleteErr))
			continue
		}
		if deleteErr := s.iconAssetRepo.DeleteClaimedModelIconAsset(ctx, item.ID); deleteErr != nil && !errors.Is(deleteErr, repository.ErrNotFound) {
			cleanupErr = errors.Join(cleanupErr, deleteErr)
		}
	}
	return cleanupErr
}
