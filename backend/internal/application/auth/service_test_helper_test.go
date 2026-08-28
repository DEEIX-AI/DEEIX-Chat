package auth

import (
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/identityprovider"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

func newTestService(cfg config.Config, repo repository.AuthRepository, geoResolver GeoResolver) *Service {
	return NewServiceWithRuntime(
		config.NewRuntime(cfg),
		repo,
		geoResolver,
		identityprovider.New(cfg.StrictOutboundPolicy()),
	)
}
