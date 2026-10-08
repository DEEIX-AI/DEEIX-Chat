// Package secretrotation 在配置了旧主密钥（DATA_ENCRYPTION_KEYS_PREVIOUS）时，
// 于启动后在后台把存量密文改用当前主密钥重新加密，完成后旧主密钥即可移除。
package secretrotation

import (
	"context"
	"errors"
	"fmt"
	"time"

	domainpersonalprovider "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/personalprovider"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/secretbox"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"go.uber.org/zap"
)

const (
	batchSize = 200
	// maxLoggedFailures 限制每个字段逐条记录的失败数；密钥配错时整表都会失败，不应刷屏。
	maxLoggedFailures = 20
)

// 按顺序轮换的字段。兑换码与系统配置有各自的流程，见 Run。
var secretFields = []repository.SecretField{
	repository.SecretFieldUpstreamAPIKeys,
	repository.SecretFieldPersonalProviderAPIKey,
	repository.SecretFieldMCPAuthToken,
	repository.SecretFieldIdentityProviderSecret,
	repository.SecretFieldTOTPSecret,
	repository.SecretFieldModerationText,
}

// secretBindings 给出密文绑定到记录的字段的绑定上下文；这些字段必须用同一上下文重新加密。
var secretBindings = map[repository.SecretField]func(item repository.StoredSecret) string{
	repository.SecretFieldPersonalProviderAPIKey: func(item repository.StoredSecret) string {
		return domainpersonalprovider.APIKeyBinding(item.OwnerUserID, item.PublicID)
	},
}

// Service 执行一次密文轮换。
type Service struct {
	repo        repository.SecretRotationRepository
	keyring     *secretbox.Keyring
	logger      *zap.Logger
	settingKeys []repository.SettingKey
	// onSettingRotated 在系统配置的密文被替换后调用，用于清除缓存中的旧密文。
	onSettingRotated func(ctx context.Context, namespace string, key string)
	now              func() time.Time
}

// NewService 创建密文轮换服务。
func NewService(repo repository.SecretRotationRepository, keyring *secretbox.Keyring, logger *zap.Logger) *Service {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{repo: repo, keyring: keyring, logger: logger, now: time.Now}
}

// SetSensitiveSettings 指定以密文存储的系统配置项，以及替换后清除缓存的回调。
func (s *Service) SetSensitiveSettings(keys []repository.SettingKey, onRotated func(ctx context.Context, namespace string, key string)) {
	s.settingKeys = keys
	s.onSettingRotated = onRotated
}

// Start 在配置了旧主密钥时于后台执行一次轮换；未配置时不做任何事。
func (s *Service) Start(ctx context.Context) {
	if s == nil || s.repo == nil || s.keyring.PreviousKeyCount() == 0 {
		return
	}
	go s.Run(ctx)
}

// fieldResult 汇总一个字段的处理结果。
type fieldResult struct {
	scanned     int
	reencrypted int
	current     int
	concurrent  int
	failed      int
	aborted     error
}

func (r *fieldResult) add(other fieldResult) {
	r.scanned += other.scanned
	r.reencrypted += other.reencrypted
	r.current += other.current
	r.concurrent += other.concurrent
	r.failed += other.failed
}

// Run 执行一次完整轮换并在日志中报告结果。各字段相互独立：一个字段出错不影响其余字段。
func (s *Service) Run(ctx context.Context) {
	started := s.now()
	s.logger.Info("data_encryption_key_rotation_started",
		zap.Int("previous_keys", s.keyring.PreviousKeyCount()),
		zap.String("detail", "Re-encrypting stored secrets that were encrypted with a key in DATA_ENCRYPTION_KEYS_PREVIOUS so that they use DATA_ENCRYPTION_KEY."),
	)

	var total fieldResult
	var abortedFields []string
	record := func(field repository.SecretField, result fieldResult) bool {
		total.add(result)
		if result.aborted != nil {
			if ctx.Err() != nil {
				s.logger.Warn("data_encryption_key_rotation_interrupted",
					zap.String("field", string(field)),
					zap.String("detail", "Key rotation stopped because the server is shutting down; it resumes at the next startup."),
				)
				return false
			}
			abortedFields = append(abortedFields, string(field))
			s.logger.Error("data_encryption_key_rotation_field_aborted",
				zap.String("field", string(field)),
				zap.Error(result.aborted),
				zap.String("detail", "Rotation of this field stopped on a database error; the remaining records are retried at the next startup."),
			)
		}
		s.logFieldFinished(field, result)
		return true
	}

	for _, field := range secretFields {
		if !record(field, s.rotateField(ctx, field)) {
			return
		}
	}
	if !record(repository.SecretFieldSystemSetting, s.rotateSettings(ctx)) {
		return
	}
	redemption, withoutPlaintext := s.rotateRedemptionCodes(ctx)
	if !record(repository.SecretFieldRedemptionCodePlaintext, redemption) {
		return
	}
	imagesExpireAt, err := s.repo.LatestIsolatedImageExpiry(ctx, s.now())
	if err != nil {
		abortedFields = append(abortedFields, string(repository.SecretFieldModerationIsolatedImages))
		s.logger.Error("data_encryption_key_rotation_field_aborted", zap.String("field", string(repository.SecretFieldModerationIsolatedImages)), zap.Error(err))
	}

	var reasons []string
	if total.failed > 0 {
		reasons = append(reasons, fmt.Sprintf("%d record(s) could not be decrypted with any configured key and were left unchanged; see data_encryption_key_rotation_record_failed.", total.failed))
	}
	if len(abortedFields) > 0 {
		reasons = append(reasons, "Some fields stopped on a database error and will be retried at the next startup; see data_encryption_key_rotation_field_aborted.")
	}
	if withoutPlaintext > 0 {
		reasons = append(reasons, fmt.Sprintf("%d redemption code(s) were created before code plaintext was stored and cannot be re-indexed; they can no longer be redeemed once the previous key is removed. Disable or replace them first.", withoutPlaintext))
	}
	if imagesExpireAt != nil {
		reasons = append(reasons, fmt.Sprintf("Isolated content moderation images are not re-encrypted and expire on their own; the last one expires at %s. Keep the previous key until then.", imagesExpireAt.UTC().Format(time.RFC3339)))
	}

	fields := []zap.Field{
		zap.Int("scanned", total.scanned),
		zap.Int("reencrypted", total.reencrypted),
		zap.Int("already_current", total.current),
		zap.Int("failed", total.failed),
		zap.Int64("duration_ms", s.now().Sub(started).Milliseconds()),
	}
	if len(reasons) == 0 {
		s.logger.Info("data_encryption_key_rotation_completed", append(fields,
			zap.String("detail", "All stored secrets are encrypted with DATA_ENCRYPTION_KEY. DATA_ENCRYPTION_KEYS_PREVIOUS can now be removed; restart the server afterwards."),
		)...)
		return
	}
	s.logger.Warn("data_encryption_key_rotation_incomplete", append(fields,
		zap.Strings("reasons", reasons),
		zap.String("detail", "Keep DATA_ENCRYPTION_KEYS_PREVIOUS configured until every reason above is resolved; removing it now makes the affected data unreadable."),
	)...)
}

func (s *Service) logFieldFinished(field repository.SecretField, result fieldResult) {
	s.logger.Info("data_encryption_key_rotation_field_finished",
		zap.String("field", string(field)),
		zap.Int("scanned", result.scanned),
		zap.Int("reencrypted", result.reencrypted),
		zap.Int("already_current", result.current),
		zap.Int("changed_concurrently", result.concurrent),
		zap.Int("failed", result.failed),
	)
}

func (s *Service) logRecordFailed(field repository.SecretField, id uint, failures int, err error) {
	if failures > maxLoggedFailures {
		return
	}
	detail := "The record could not be re-encrypted and was left unchanged."
	if failures == maxLoggedFailures {
		detail += fmt.Sprintf(" Further failures for this field are counted but not logged individually (limit %d).", maxLoggedFailures)
	}
	s.logger.Warn("data_encryption_key_rotation_record_failed",
		zap.String("field", string(field)),
		zap.Uint("record_id", id),
		zap.Error(err),
		zap.String("detail", detail),
	)
}

// rotateValue 重新加密一个载荷并按条件写回。无法解密的记录计入 failed 并保持原样；
// 只有数据库错误会返回 error，调用方据此中止该字段（下次启动重试）。
// binding 非空时按 secretbox 的绑定载荷处理。
func (s *Service) rotateValue(ctx context.Context, field repository.SecretField, id uint, payload string, binding string, result *fieldResult, onReplaced func()) error {
	result.scanned++
	var next string
	var changed bool
	var err error
	if binding != "" {
		next, changed, err = s.keyring.ReencryptBound(payload, binding)
	} else {
		next, changed, err = s.keyring.Reencrypt(payload)
	}
	if err != nil {
		result.failed++
		s.logRecordFailed(field, id, result.failed, err)
		return nil
	}
	if !changed {
		result.current++
		return nil
	}
	replaced, err := s.repo.ReplaceSecret(ctx, field, id, payload, next)
	if err != nil {
		return err
	}
	if !replaced {
		// 读取后被其他实例或用户改写；写入方使用的必然是当前主密钥，无需再处理。
		result.concurrent++
		return nil
	}
	result.reencrypted++
	if onReplaced != nil {
		onReplaced()
	}
	return nil
}

func (s *Service) rotateField(ctx context.Context, field repository.SecretField) fieldResult {
	var result fieldResult
	var afterID uint
	for {
		if err := ctx.Err(); err != nil {
			result.aborted = err
			return result
		}
		page, err := s.repo.ListSecrets(ctx, field, afterID, batchSize)
		if err != nil {
			result.aborted = err
			return result
		}
		for _, item := range page {
			binding := ""
			if bind, ok := secretBindings[field]; ok {
				binding = bind(item)
			}
			if err := s.rotateValue(ctx, field, item.ID, item.Value, binding, &result, nil); err != nil {
				result.aborted = err
				return result
			}
			afterID = item.ID
		}
		if len(page) < batchSize {
			return result
		}
	}
}

func (s *Service) rotateSettings(ctx context.Context) fieldResult {
	var result fieldResult
	items, err := s.repo.ListSettingSecrets(ctx, s.settingKeys)
	if err != nil {
		result.aborted = err
		return result
	}
	for _, item := range items {
		err := s.rotateValue(ctx, repository.SecretFieldSystemSetting, item.ID, item.Value, "", &result, func() {
			if s.onSettingRotated != nil {
				s.onSettingRotated(ctx, item.Namespace, item.Key)
			}
		})
		if err != nil {
			result.aborted = err
			return result
		}
	}
	return result
}

// rotateRedemptionCodes 重新加密兑换码原文，并用当前主密钥重算查找索引；两者一起写回。
// 第二个返回值是未保存原文、无法重算索引的早期兑换码数量。
func (s *Service) rotateRedemptionCodes(ctx context.Context) (fieldResult, int) {
	var result fieldResult
	withoutPlaintext := 0
	var afterID uint
	for {
		if err := ctx.Err(); err != nil {
			result.aborted = err
			return result, withoutPlaintext
		}
		page, err := s.repo.ListRedemptionCodes(ctx, afterID, batchSize)
		if err != nil {
			result.aborted = err
			return result, withoutPlaintext
		}
		for _, item := range page {
			afterID = item.ID
			if item.CodeEncrypted == "" {
				withoutPlaintext++
				continue
			}
			if err := s.rotateRedemptionCode(ctx, item, &result); err != nil {
				result.aborted = err
				return result, withoutPlaintext
			}
		}
		if len(page) < batchSize {
			return result, withoutPlaintext
		}
	}
}

// rotateRedemptionCode 的错误语义同 rotateValue：只有数据库错误会返回 error。
func (s *Service) rotateRedemptionCode(ctx context.Context, item repository.StoredRedemptionCode, result *fieldResult) error {
	const field = repository.SecretFieldRedemptionCodePlaintext
	result.scanned++
	plaintext, current, err := s.keyring.Open(item.CodeEncrypted)
	if err == nil && len(plaintext) == 0 {
		err = errors.New("redemption code plaintext is empty")
	}
	if err != nil {
		result.failed++
		s.logRecordFailed(field, item.ID, result.failed, err)
		return nil
	}
	nextHash := s.keyring.LookupHashes(string(plaintext))[0]
	if current && item.CodeHash == nextHash {
		result.current++
		return nil
	}
	nextEncrypted := item.CodeEncrypted
	if !current {
		if nextEncrypted, _, err = s.keyring.Reencrypt(item.CodeEncrypted); err != nil {
			result.failed++
			s.logRecordFailed(field, item.ID, result.failed, err)
			return nil
		}
	}
	replaced, err := s.repo.ReplaceRedemptionCode(ctx, item.ID, item, nextHash, nextEncrypted)
	if err != nil {
		return err
	}
	if replaced {
		result.reencrypted++
	} else {
		result.concurrent++
	}
	return nil
}
