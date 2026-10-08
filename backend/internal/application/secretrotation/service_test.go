package secretrotation

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/secretbox"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

const (
	newKey = "new-data-encryption-key-0123456789abcdef"
	oldKey = "old-data-encryption-key-0123456789abcdef"
)

type memoryRepo struct {
	mu          sync.Mutex
	secrets     map[repository.SecretField]map[uint]string
	settings    []repository.StoredSettingSecret
	codes       map[uint]repository.StoredRedemptionCode
	imageExpiry *time.Time
	listErr     map[repository.SecretField]error
	// conflict 让对应字段的条件替换总是失败，模拟其他实例已改写该记录。
	conflict map[repository.SecretField]bool
	// owners 与 publicIDs 是密文绑定到记录的字段所需的上下文（按记录 ID）。
	owners    map[uint]uint
	publicIDs map[uint]string
	calls     int
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{
		secrets:  map[repository.SecretField]map[uint]string{},
		codes:    map[uint]repository.StoredRedemptionCode{},
		listErr:  map[repository.SecretField]error{},
		conflict: map[repository.SecretField]bool{},
	}
}

func (r *memoryRepo) ListSecrets(_ context.Context, field repository.SecretField, afterID uint, limit int) ([]repository.StoredSecret, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if err := r.listErr[field]; err != nil {
		return nil, err
	}
	var ids []uint
	for id, value := range r.secrets[field] {
		if id > afterID && value != "" {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	var page []repository.StoredSecret
	for _, id := range ids {
		if len(page) == limit {
			break
		}
		page = append(page, repository.StoredSecret{ID: id, Value: r.secrets[field][id], OwnerUserID: r.owners[id], PublicID: r.publicIDs[id]})
	}
	return page, nil
}

func (r *memoryRepo) ReplaceSecret(_ context.Context, field repository.SecretField, id uint, previous string, next string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if field == repository.SecretFieldSystemSetting {
		for index := range r.settings {
			if r.settings[index].ID == id && r.settings[index].Value == previous {
				r.settings[index].Value = next
				return true, nil
			}
		}
		return false, nil
	}
	if r.conflict[field] || r.secrets[field][id] != previous {
		return false, nil
	}
	r.secrets[field][id] = next
	return true, nil
}

func (r *memoryRepo) ListSettingSecrets(context.Context, []repository.SettingKey) ([]repository.StoredSettingSecret, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]repository.StoredSettingSecret(nil), r.settings...), nil
}

func (r *memoryRepo) ListRedemptionCodes(_ context.Context, afterID uint, limit int) ([]repository.StoredRedemptionCode, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var page []repository.StoredRedemptionCode
	for id := afterID + 1; id <= afterID+uint(limit)+100 && len(page) < limit; id++ {
		if code, ok := r.codes[id]; ok {
			page = append(page, code)
		}
	}
	return page, nil
}

func (r *memoryRepo) ReplaceRedemptionCode(_ context.Context, id uint, previous repository.StoredRedemptionCode, nextHash string, nextEncrypted string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	current := r.codes[id]
	if current.CodeHash != previous.CodeHash || current.CodeEncrypted != previous.CodeEncrypted {
		return false, nil
	}
	r.codes[id] = repository.StoredRedemptionCode{ID: id, CodeHash: nextHash, CodeEncrypted: nextEncrypted}
	return true, nil
}

func (r *memoryRepo) LatestIsolatedImageExpiry(context.Context, time.Time) (*time.Time, error) {
	return r.imageExpiry, nil
}

func keyring(t *testing.T, active string, previous ...string) *secretbox.Keyring {
	t.Helper()
	ring, err := secretbox.NewKeyring(active, previous...)
	if err != nil {
		t.Fatal(err)
	}
	return ring
}

// secretPayload 按字段的存储方式加密：绑定到记录的字段使用 secretBindings 给出的上下文。
func secretPayload(t *testing.T, repo *memoryRepo, field repository.SecretField, id uint, key string, value string) string {
	t.Helper()
	bind, ok := secretBindings[field]
	if !ok {
		return encrypt(t, key, value)
	}
	payload, err := keyring(t, key).EncryptStringBound(value, bind(repository.StoredSecret{OwnerUserID: repo.owners[id], PublicID: repo.publicIDs[id]}))
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func readSecret(t *testing.T, ring *secretbox.Keyring, repo *memoryRepo, field repository.SecretField, id uint) (string, error) {
	t.Helper()
	bind, ok := secretBindings[field]
	if !ok {
		return ring.DecryptString(repo.secrets[field][id])
	}
	return ring.DecryptStringBound(repo.secrets[field][id], bind(repository.StoredSecret{OwnerUserID: repo.owners[id], PublicID: repo.publicIDs[id]}))
}

func encrypt(t *testing.T, key string, value string) string {
	t.Helper()
	payload, err := keyring(t, key).EncryptString(value)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func newObservedService(t *testing.T, repo *memoryRepo) (*Service, *observer.ObservedLogs) {
	t.Helper()
	core, logs := observer.New(zapcore.InfoLevel)
	return NewService(repo, keyring(t, newKey, oldKey), zap.New(core)), logs
}

func onlyEntry(t *testing.T, logs *observer.ObservedLogs, message string) observer.LoggedEntry {
	t.Helper()
	entries := logs.FilterMessage(message).All()
	if len(entries) != 1 {
		t.Fatalf("expected one %q entry, got %d", message, len(entries))
	}
	return entries[0]
}

func TestRunReencryptsEveryFieldWithTheCurrentKey(t *testing.T) {
	repo := newMemoryRepo()
	repo.owners = map[uint]uint{1: 7, 2: 8}
	repo.publicIDs = map[uint]string{1: "providerone", 2: "providertwo"}
	for _, field := range secretFields {
		repo.secrets[field] = map[uint]string{
			1: secretPayload(t, repo, field, 1, oldKey, "old-"+string(field)),
			2: secretPayload(t, repo, field, 2, newKey, "new-"+string(field)),
		}
	}
	repo.settings = []repository.StoredSettingSecret{{ID: 9, Namespace: "auth", Key: "smtp_password", Value: encrypt(t, oldKey, "smtp-secret")}}
	oldIndex := keyring(t, oldKey).LookupHashes("CODE-2026")[0]
	repo.codes[1] = repository.StoredRedemptionCode{ID: 1, CodeHash: oldIndex, CodeEncrypted: encrypt(t, oldKey, "CODE-2026")}

	service, logs := newObservedService(t, repo)
	var evicted []string
	service.SetSensitiveSettings(nil, func(_ context.Context, namespace string, key string) {
		evicted = append(evicted, namespace+"."+key)
	})
	service.Run(context.Background())

	current := keyring(t, newKey)
	for _, field := range secretFields {
		for id, want := range map[uint]string{1: "old-" + string(field), 2: "new-" + string(field)} {
			got, err := readSecret(t, current, repo, field, id)
			if err != nil || got != want {
				t.Fatalf("%s #%d: %q, %v", field, id, got, err)
			}
		}
	}
	if got, err := current.DecryptString(repo.settings[0].Value); err != nil || got != "smtp-secret" {
		t.Fatalf("setting: %q, %v", got, err)
	}
	if len(evicted) != 1 || evicted[0] != "auth.smtp_password" {
		t.Fatalf("the cached setting must be evicted, got %v", evicted)
	}
	code := repo.codes[1]
	if code.CodeHash != current.LookupHashes("CODE-2026")[0] {
		t.Fatal("the redemption index must be recomputed with the current key")
	}
	if got, err := current.DecryptString(code.CodeEncrypted); err != nil || got != "CODE-2026" {
		t.Fatalf("redemption code: %q, %v", got, err)
	}

	entry := onlyEntry(t, logs, "data_encryption_key_rotation_completed")
	fields := entry.ContextMap()
	if fields["reencrypted"] != int64(len(secretFields)+2) || fields["already_current"] != int64(len(secretFields)) || fields["failed"] != int64(0) {
		t.Fatalf("summary = %v", fields)
	}
	if !strings.Contains(fields["detail"].(string), "DATA_ENCRYPTION_KEYS_PREVIOUS can now be removed") {
		t.Fatalf("detail = %q", fields["detail"])
	}
	if logs.FilterMessage("data_encryption_key_rotation_incomplete").Len() != 0 {
		t.Fatal("a complete rotation must not report itself as incomplete")
	}

	// 再次运行时一切都已是当前密钥，不应再写入。
	logs.TakeAll()
	service.Run(context.Background())
	if fields := onlyEntry(t, logs, "data_encryption_key_rotation_completed").ContextMap(); fields["reencrypted"] != int64(0) {
		t.Fatalf("a second run re-encrypted again: %v", fields)
	}
}

func TestRunLeavesUndecryptableDataAndReportsWhyThePreviousKeyIsStillNeeded(t *testing.T) {
	repo := newMemoryRepo()
	foreign := encrypt(t, "unrelated-data-encryption-key-0123456789", "lost")
	repo.secrets[repository.SecretFieldMCPAuthToken] = map[uint]string{5: foreign}
	repo.codes[3] = repository.StoredRedemptionCode{ID: 3, CodeHash: "legacy-index"}
	expiry := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	repo.imageExpiry = &expiry

	service, logs := newObservedService(t, repo)
	service.Run(context.Background())

	if repo.secrets[repository.SecretFieldMCPAuthToken][5] != foreign {
		t.Fatal("data that cannot be decrypted must be left unchanged")
	}
	failed := onlyEntry(t, logs, "data_encryption_key_rotation_record_failed").ContextMap()
	if failed["field"] != string(repository.SecretFieldMCPAuthToken) || failed["record_id"] != uint64(5) {
		t.Fatalf("failure entry = %v", failed)
	}
	entry := onlyEntry(t, logs, "data_encryption_key_rotation_incomplete")
	if entry.Level != zapcore.WarnLevel {
		t.Fatalf("level = %s", entry.Level)
	}
	reasons := strings.Join(entryStrings(entry, "reasons"), "\n")
	for _, want := range []string{"1 record(s) could not be decrypted", "1 redemption code(s) were created before", "expires at 2026-11-01T00:00:00Z"} {
		if !strings.Contains(reasons, want) {
			t.Fatalf("reasons %q lack %q", reasons, want)
		}
	}
	if logs.FilterMessage("data_encryption_key_rotation_completed").Len() != 0 {
		t.Fatal("an incomplete rotation must not report completion")
	}
}

func TestRunCountsRecordsChangedByAnotherWriter(t *testing.T) {
	repo := newMemoryRepo()
	repo.secrets[repository.SecretFieldUpstreamAPIKeys] = map[uint]string{1: encrypt(t, oldKey, "k")}
	repo.conflict[repository.SecretFieldUpstreamAPIKeys] = true

	service, logs := newObservedService(t, repo)
	service.Run(context.Background())

	for _, entry := range logs.FilterMessage("data_encryption_key_rotation_field_finished").All() {
		if entry.ContextMap()["field"] == string(repository.SecretFieldUpstreamAPIKeys) {
			if fields := entry.ContextMap(); fields["changed_concurrently"] != int64(1) || fields["failed"] != int64(0) {
				t.Fatalf("field summary = %v", fields)
			}
			return
		}
	}
	t.Fatal("missing the upstream field summary")
}

func TestRunContinuesAfterADatabaseError(t *testing.T) {
	repo := newMemoryRepo()
	repo.listErr[repository.SecretFieldUpstreamAPIKeys] = errors.New("connection reset")
	repo.secrets[repository.SecretFieldMCPAuthToken] = map[uint]string{1: encrypt(t, oldKey, "t")}

	service, logs := newObservedService(t, repo)
	service.Run(context.Background())

	if aborted := onlyEntry(t, logs, "data_encryption_key_rotation_field_aborted"); aborted.Level != zapcore.ErrorLevel {
		t.Fatalf("level = %s", aborted.Level)
	}
	if got, _ := keyring(t, newKey).DecryptString(repo.secrets[repository.SecretFieldMCPAuthToken][1]); got != "t" {
		t.Fatal("later fields must still be rotated")
	}
	reasons := strings.Join(entryStrings(onlyEntry(t, logs, "data_encryption_key_rotation_incomplete"), "reasons"), "\n")
	if !strings.Contains(reasons, "retried at the next startup") {
		t.Fatalf("reasons = %q", reasons)
	}
}

func TestRunStopsQuietlyOnShutdown(t *testing.T) {
	repo := newMemoryRepo()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service, logs := newObservedService(t, repo)
	service.Run(ctx)
	onlyEntry(t, logs, "data_encryption_key_rotation_interrupted")
	if logs.FilterMessage("data_encryption_key_rotation_completed").Len()+logs.FilterMessage("data_encryption_key_rotation_incomplete").Len() != 0 {
		t.Fatal("an interrupted run must not report a result")
	}
}

func TestStartDoesNothingWithoutPreviousKeys(t *testing.T) {
	repo := newMemoryRepo()
	NewService(repo, keyring(t, newKey), zap.NewNop()).Start(context.Background())
	time.Sleep(50 * time.Millisecond)
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if repo.calls != 0 {
		t.Fatalf("rotation ran without DATA_ENCRYPTION_KEYS_PREVIOUS (%d calls)", repo.calls)
	}
}

func entryStrings(entry observer.LoggedEntry, key string) []string {
	for _, field := range entry.Context {
		if field.Key == key {
			if values, ok := field.Interface.(zapcore.ArrayMarshaler); ok {
				encoder := zapcore.NewMapObjectEncoder()
				_ = encoder.AddArray(key, values)
				var result []string
				for _, value := range encoder.Fields[key].([]any) {
					result = append(result, value.(string))
				}
				return result
			}
		}
	}
	return nil
}

// 密钥配错时整张表都会失败：逐条日志有上限，但计数完整。
func TestRecordFailuresAreCappedInTheLog(t *testing.T) {
	repo := newMemoryRepo()
	values := map[uint]string{}
	for id := uint(1); id <= maxLoggedFailures+5; id++ {
		values[id] = encrypt(t, "unrelated-data-encryption-key-0123456789", "x")
	}
	repo.secrets[repository.SecretFieldUpstreamAPIKeys] = values

	service, logs := newObservedService(t, repo)
	service.Run(context.Background())

	failures := logs.FilterMessage("data_encryption_key_rotation_record_failed").All()
	if len(failures) != maxLoggedFailures {
		t.Fatalf("logged %d failures, want %d", len(failures), maxLoggedFailures)
	}
	if !strings.Contains(failures[len(failures)-1].ContextMap()["detail"].(string), "not logged individually") {
		t.Fatal("the last logged failure must say that further failures are not logged")
	}
	if got := onlyEntry(t, logs, "data_encryption_key_rotation_incomplete").ContextMap()["failed"]; got != int64(maxLoggedFailures+5) {
		t.Fatalf("failed = %v", got)
	}
}
