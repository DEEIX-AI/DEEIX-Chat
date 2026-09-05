package channel

import (
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

func newTestService(
	cfg config.Config,
	repo repository.ChannelRepository,
	presentationRepo repository.ModelPresentationRepository,
	cache repository.ChannelCacheRepository,
	llmClient llmGateway,
) *Service {
	return NewServiceWithRuntime(config.NewRuntime(cfg), repo, presentationRepo, cache, llmClient)
}
