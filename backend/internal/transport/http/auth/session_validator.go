package auth

import (
	"context"
	"time"

	appauth "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/auth"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/requestmeta"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
)

// SessionValidator adapts the auth application service to the HTTP session gate.
type SessionValidator struct {
	Service *appauth.Service
}

// ValidateAccessSession implements middleware.SessionValidator.
func (v SessionValidator) ValidateAccessSession(
	ctx context.Context,
	userID uint,
	sessionID string,
	accessIssuedAt time.Time,
	auditCtx requestmeta.SessionAuditContext,
) (middleware.AccessSessionState, error) {
	if v.Service == nil {
		return middleware.AccessSessionState{}, appauth.ErrSessionRevoked
	}
	state, err := v.Service.ValidateAccessSession(ctx, userID, sessionID, accessIssuedAt, auditCtx)
	if err != nil {
		return middleware.AccessSessionState{}, err
	}
	return middleware.AccessSessionState{
		Role:                    state.Role,
		InitialSecurityRequired: state.InitialSecurityRequired,
	}, nil
}
