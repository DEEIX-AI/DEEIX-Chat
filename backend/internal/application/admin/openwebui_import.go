package admin

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/textutil"
	"math"
	"strings"
	"time"

	auditapp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/audit"
	userapp "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/user"
	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

const (
	openWebUIImportSource       = "openwebui"
	openWebUIImportDedupeField  = "email"
	openWebUIImportDedupeRule   = "Existing users are matched by email and skipped without modification."
	openWebUIAppearanceDefaults = `{"chatFont":"default","chatFontWeight":"regular","fontSize":"standard","preset":"default","theme":"system"}`
	openWebUIPasswordHashCost   = 12
)

type openWebUIImportCandidate struct {
	sourceKey      string
	email          string
	lowerEmail     string
	displayName    string
	balanceNanousd int64
}

type openWebUIImportUserService interface {
	ListUsersByLowerEmails(ctx context.Context, emails []string) (map[string]domainuser.User, error)
	ListAllUsernames(ctx context.Context) ([]string, error)
	ImportUsersWithCredentialsAndBalances(ctx context.Context, records []repository.UserImportRecord) ([]domainuser.User, error)
}

type openWebUIRowLoader interface {
	LoadOpenWebUIRows(ctx context.Context, dsn string) ([]repository.OpenWebUIUserRow, error)
}

// ImportOpenWebUIUsers 从 OpenWebUI 数据库导入用户。
func (s *Service) ImportOpenWebUIUsers(
	ctx context.Context,
	requestID string,
	actorUserID uint,
	input OpenWebUIImportInput,
	ip string,
	userAgent string,
) (*OpenWebUIImportResult, error) {
	if _, err := s.getActorUser(ctx, actorUserID); err != nil {
		return nil, err
	}
	if !validCreditMultiplier(input.CreditMultiplier) {
		return nil, ErrInvalidImportMultiplier
	}
	importUserService, ok := s.userService.(openWebUIImportUserService)
	if !ok {
		return nil, ErrOpenWebUIImportFailed
	}
	if s.openWebUIRowLoader == nil {
		return nil, ErrOpenWebUIImportFailed
	}

	rows, err := s.openWebUIRowLoader.LoadOpenWebUIRows(ctx, input.DSN)
	if err != nil {
		if errors.Is(err, repository.ErrInvalidInput) {
			return nil, ErrInvalidImportDSN
		}
		return nil, ErrOpenWebUIImportFailed
	}

	result := &OpenWebUIImportResult{
		Source:      openWebUIImportSource,
		DedupeField: openWebUIImportDedupeField,
		DedupeRule:  openWebUIImportDedupeRule,
		Scanned:     len(rows),
	}
	if len(rows) == 0 {
		if !input.DryRun {
			s.writeOpenWebUIImportAudit(ctx, requestID, actorUserID, ip, userAgent, result)
		}
		return result, nil
	}

	candidates, emailKeys := s.buildOpenWebUIImportCandidates(rows, input.CreditMultiplier, result)
	if len(candidates) == 0 {
		if !input.DryRun {
			s.writeOpenWebUIImportAudit(ctx, requestID, actorUserID, ip, userAgent, result)
		}
		return result, nil
	}

	existingByEmail, err := importUserService.ListUsersByLowerEmails(ctx, emailKeys)
	if err != nil {
		return nil, err
	}
	if input.DryRun {
		for _, candidate := range candidates {
			if _, exists := existingByEmail[candidate.lowerEmail]; exists {
				result.SkippedExistingEmail++
				continue
			}
			result.Imported++
		}
		return result, nil
	}

	usernames, err := importUserService.ListAllUsernames(ctx)
	if err != nil {
		return nil, err
	}
	usedUsernames := make(map[string]struct{}, len(usernames)+len(candidates))
	for _, username := range usernames {
		value := strings.ToLower(strings.TrimSpace(username))
		if value != "" {
			usedUsernames[value] = struct{}{}
		}
	}

	records := make([]repository.UserImportRecord, 0, len(candidates))
	now := time.Now()
	for _, candidate := range candidates {
		if _, exists := existingByEmail[candidate.lowerEmail]; exists {
			result.SkippedExistingEmail++
			continue
		}

		passwordHash, hashErr := bcrypt.GenerateFromPassword([]byte(randomImportPassword()), openWebUIPasswordHashCost)
		if hashErr != nil {
			return nil, hashErr
		}
		username := resolveOpenWebUIUsername(candidate.sourceKey, candidate.lowerEmail, usedUsernames)
		records = append(records, repository.UserImportRecord{
			User: domainuser.User{
				PublicID:              resolveOpenWebUIPublicID(candidate.sourceKey, candidate.lowerEmail),
				Username:              username,
				DisplayName:           candidate.displayName,
				Email:                 candidate.email,
				EmailSource:           domainuser.EmailSourceProviderVerified,
				Role:                  domainuser.RoleUser,
				Status:                domainuser.StatusActive,
				Timezone:              "Asia/Shanghai",
				Locale:                "zh-CN",
				AppearancePreferences: openWebUIAppearanceDefaults,
			},
			Credential: domainuser.Credential{
				PasswordHash:      string(passwordHash),
				PasswordAlgo:      "bcrypt",
				PasswordEnabled:   true,
				PasswordUpdatedAt: &now,
				PasswordSetAt:     &now,
				PasswordOrigin:    domainuser.PasswordOriginAdminCreated,
			},
			BillingBalanceNanousd:     candidate.balanceNanousd,
			BillingBalanceRefNo:       "openwebui_import",
			BillingBalanceDescription: "OpenWebUI import",
		})
	}

	if len(records) > 0 {
		if _, err = importUserService.ImportUsersWithCredentialsAndBalances(ctx, records); err != nil {
			return nil, err
		}
	}
	result.Imported = len(records)
	s.writeOpenWebUIImportAudit(ctx, requestID, actorUserID, ip, userAgent, result)
	return result, nil
}

func (s *Service) buildOpenWebUIImportCandidates(rows []repository.OpenWebUIUserRow, multiplier float64, result *OpenWebUIImportResult) ([]openWebUIImportCandidate, []string) {
	candidates := make([]openWebUIImportCandidate, 0, len(rows))
	emailKeys := make([]string, 0, len(rows))
	seenSourceEmails := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		email, err := userapp.NormalizeEmail(row.Email)
		lowerEmail := strings.ToLower(strings.TrimSpace(email))
		if err != nil || lowerEmail == "" {
			result.SkippedInvalidEmail++
			continue
		}
		if _, exists := seenSourceEmails[lowerEmail]; exists {
			result.SkippedDuplicateSourceEmail++
			continue
		}
		seenSourceEmails[lowerEmail] = struct{}{}
		sourceKey := textutil.FirstNonEmpty(row.PublicID, row.Username, lowerEmail)
		if sourceKey == "" {
			result.SkippedInvalidRow++
			continue
		}
		candidates = append(candidates, openWebUIImportCandidate{
			sourceKey:      sourceKey,
			email:          email,
			lowerEmail:     lowerEmail,
			displayName:    userapp.NormalizeGeneratedDisplayName(row.DisplayName),
			balanceNanousd: openWebUICreditToNanousd(row.Balance, multiplier),
		})
		emailKeys = append(emailKeys, lowerEmail)
	}
	return candidates, emailKeys
}

func validCreditMultiplier(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func openWebUICreditToNanousd(credit float64, multiplier float64) int64 {
	if credit <= 0 || multiplier <= 0 || math.IsNaN(credit) || math.IsInf(credit, 0) {
		return 0
	}
	value := credit * multiplier * 1_000_000_000
	if value <= 0 {
		return 0
	}
	const maxInt64 = int64(^uint64(0) >> 1)
	if value >= float64(maxInt64) {
		return maxInt64
	}
	return int64(math.Round(value))
}

func resolveOpenWebUIPublicID(sourceKey string, lowerEmail string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(sourceKey) + "\x00" + strings.TrimSpace(lowerEmail)))
	return "ow" + hex.EncodeToString(sum[:])[:30]
}

func resolveOpenWebUIUsername(sourceKey string, lowerEmail string, used map[string]struct{}) string {
	hash := shortOpenWebUIHash(sourceKey, lowerEmail)
	base := sanitizeOpenWebUIUsername(sourceKey)
	if _, err := userapp.NormalizeUsername(base); err != nil {
		base = "owui-" + hash[:8]
	}
	if len(base) > userapp.UsernameMaxLength {
		base = strings.Trim(base[:userapp.UsernameMaxLength], "-_")
	}
	if _, err := userapp.NormalizeUsername(base); err != nil {
		base = "owui-" + hash[:8]
	}
	if reserveUsername(base, used) {
		return base
	}

	prefix := base
	if len(prefix) > userapp.UsernameMaxLength-5 {
		prefix = strings.Trim(prefix[:userapp.UsernameMaxLength-5], "-_")
	}
	for offset := 0; offset+4 <= len(hash); offset += 4 {
		candidate := strings.Trim(prefix, "-_") + "-" + hash[offset:offset+4]
		if _, err := userapp.NormalizeUsername(candidate); err == nil && reserveUsername(candidate, used) {
			return candidate
		}
	}
	for offset := 0; offset+11 <= len(hash); offset++ {
		candidate := "owui-" + hash[offset:offset+11]
		if reserveUsername(candidate, used) {
			return candidate
		}
	}
	return "owui-" + hash[:11]
}

func sanitizeOpenWebUIUsername(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false
	for _, char := range value {
		valid := (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_'
		if valid {
			builder.WriteRune(char)
			lastDash = false
			continue
		}
		if char == '-' || char == ' ' || char == '.' || char == ':' || char == '/' {
			if !lastDash {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(builder.String(), "-_")
}

func reserveUsername(username string, used map[string]struct{}) bool {
	key := strings.ToLower(strings.TrimSpace(username))
	if key == "" {
		return false
	}
	if _, exists := used[key]; exists {
		return false
	}
	used[key] = struct{}{}
	return true
}

func shortOpenWebUIHash(sourceKey string, lowerEmail string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(sourceKey) + "\x00" + strings.TrimSpace(lowerEmail)))
	return hex.EncodeToString(sum[:])
}

func randomImportPassword() string {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		sum := sha256.Sum256([]byte(time.Now().Format(time.RFC3339Nano)))
		return base64.RawURLEncoding.EncodeToString(sum[:])
	}
	return base64.RawURLEncoding.EncodeToString(buf[:])
}

func (s *Service) writeOpenWebUIImportAudit(
	ctx context.Context,
	requestID string,
	actorUserID uint,
	ip string,
	userAgent string,
	result *OpenWebUIImportResult,
) {
	if result == nil {
		return
	}
	s.auditService.Write(ctx, auditapp.WriteInput{
		RequestID:   requestID,
		ActorUserID: actorUserID,
		Action:      "admin_import_openwebui_users",
		Resource:    "user",
		ResourceID:  openWebUIImportSource,
		IP:          ip,
		UserAgent:   userAgent,
		Detail: map[string]any{
			"source":                         result.Source,
			"dedupe_field":                   result.DedupeField,
			"scanned":                        result.Scanned,
			"imported":                       result.Imported,
			"skipped_existing_email":         result.SkippedExistingEmail,
			"skipped_duplicate_source_email": result.SkippedDuplicateSourceEmail,
			"skipped_invalid_email":          result.SkippedInvalidEmail,
			"skipped_invalid_row":            result.SkippedInvalidRow,
		},
	})
}
