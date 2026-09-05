package audit

import (
	"context"
	"testing"

	domainaudit "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/audit"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"go.uber.org/zap"
)

type auditRepositoryStub struct {
	created *domainaudit.Log
}

func (stub *auditRepositoryStub) Create(_ context.Context, item *domainaudit.Log) error {
	copy := *item
	stub.created = &copy
	return nil
}

func (*auditRepositoryStub) List(context.Context, int, int, repository.AuditLogListFilter) ([]domainaudit.Log, int64, error) {
	return nil, 0, nil
}

func TestWriteNormalizesAndPersistsStructuredInput(t *testing.T) {
	repo := &auditRepositoryStub{}
	service := NewService(repo, zap.NewNop())

	service.Write(context.Background(), WriteInput{
		RequestID:   " req_1 ",
		ActorUserID: 42,
		Action:      " user.update ",
		Resource:    " user ",
		ResourceID:  " 7 ",
		IP:          " 127.0.0.1 ",
		UserAgent:   " browser ",
		Detail:      map[string]string{"field": "status"},
	})

	if repo.created == nil {
		t.Fatal("expected audit log to be persisted")
	}
	if repo.created.RequestID != "req_1" || repo.created.Action != "user.update" ||
		repo.created.Resource != "user" || repo.created.ResourceID != "7" ||
		repo.created.IP != "127.0.0.1" || repo.created.UserAgent != "browser" {
		t.Fatalf("unexpected normalized audit log: %#v", repo.created)
	}
	if repo.created.DetailJSON != `{"field":"status"}` {
		t.Fatalf("DetailJSON = %q", repo.created.DetailJSON)
	}
}
