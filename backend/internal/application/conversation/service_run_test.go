package conversation

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"go.uber.org/zap"
)

type conversationRunClaimRepositoryStub struct {
	repository.ConversationRepository

	mu       sync.Mutex
	runs     map[string]model.Run
	claimErr error

	conversation      model.Conversation
	pairCreateErr     error
	pairCreateCalls   int
	branchCreateCalls int
}

func (r *conversationRunClaimRepositoryStub) CreateConversationRun(_ context.Context, run *model.Run) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.claimErr != nil {
		return r.claimErr
	}
	if _, exists := r.runs[run.RunID]; exists {
		return repository.ErrDuplicate
	}
	r.runs[run.RunID] = *run
	return nil
}

func (r *conversationRunClaimRepositoryStub) GetConversationByUser(_ context.Context, conversationID uint, userID uint) (*model.Conversation, error) {
	if r.conversation.ID != conversationID || r.conversation.UserID != userID {
		return nil, repository.ErrNotFound
	}
	conversation := r.conversation
	return &conversation, nil
}

func (r *conversationRunClaimRepositoryStub) ListLatestBranchPreviewMessages(context.Context, uint, int, int) ([]model.Message, error) {
	return nil, nil
}

func (r *conversationRunClaimRepositoryStub) CreateMessagePairWithUserAttachments(context.Context, *model.Message, *model.Message, []model.Attachment) error {
	r.pairCreateCalls++
	return r.pairCreateErr
}

func (r *conversationRunClaimRepositoryStub) UpdateConversationRun(_ context.Context, run *model.Run) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runs[run.RunID] = *run
	return nil
}

func (r *conversationRunClaimRepositoryStub) CreateAssistantBranchMessage(context.Context, *model.Message) error {
	r.branchCreateCalls++
	return nil
}

type conversationRunClaimRouteResolver struct {
	resolveCalls int
}

func (r *conversationRunClaimRouteResolver) ResolveRoute(context.Context, channel.ResolveRouteInput) (*channel.ResolvedRoute, error) {
	r.resolveCalls++
	return nil, errors.New("route resolution must not run after a duplicate claim")
}

func (*conversationRunClaimRouteResolver) MarkRouteFailure(context.Context, *channel.ResolvedRoute, error) {
}
func (*conversationRunClaimRouteResolver) MarkRouteSuccess(context.Context, *channel.ResolvedRoute) {}

type conversationRunClaimLLMGateway struct {
	generateCalls int
}

func (g *conversationRunClaimLLMGateway) Generate(context.Context, llm.RouteConfig, llm.GenerateInput) (*llm.GenerateOutput, error) {
	g.generateCalls++
	return nil, errors.New("upstream must not run after a duplicate claim")
}

func (g *conversationRunClaimLLMGateway) GenerateStream(context.Context, llm.RouteConfig, llm.GenerateInput, func(llm.GenerateStreamEvent) error) (*llm.GenerateOutput, error) {
	g.generateCalls++
	return nil, errors.New("upstream must not run after a duplicate claim")
}

func (g *conversationRunClaimLLMGateway) RetrieveOpenAIResponse(context.Context, llm.RouteConfig, string) (*llm.GenerateOutput, error) {
	g.generateCalls++
	return nil, errors.New("upstream must not run after a duplicate claim")
}

func (g *conversationRunClaimLLMGateway) CancelOpenAIResponse(context.Context, llm.RouteConfig, string) (*llm.GenerateOutput, error) {
	g.generateCalls++
	return nil, errors.New("upstream must not run after a duplicate claim")
}

func TestClaimConversationRunAllowsOneConcurrentOwner(t *testing.T) {
	repo := &conversationRunClaimRepositoryStub{runs: make(map[string]model.Run)}
	service := &Service{repo: repo}

	const callers = 32
	start := make(chan struct{})
	results := make(chan error, callers)
	var waitGroup sync.WaitGroup
	for range callers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			results <- service.claimConversationRun(context.Background(), &model.Run{
				RunID:          "run_concurrent_claim",
				UserID:         7,
				ConversationID: 11,
				Status:         "running",
				StartedAt:      time.Now(),
			})
		}()
	}
	close(start)
	waitGroup.Wait()
	close(results)

	successes := 0
	duplicates := 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrDuplicateMessageGenerationRun):
			duplicates++
		default:
			t.Fatalf("unexpected claim error: %v", err)
		}
	}
	if successes != 1 || duplicates != callers-1 {
		t.Fatalf("claim results = success:%d duplicate:%d", successes, duplicates)
	}
}

func TestClaimConversationRunPreservesRepositoryFailure(t *testing.T) {
	want := errors.New("database unavailable")
	service := &Service{repo: &conversationRunClaimRepositoryStub{claimErr: want}}

	err := service.claimConversationRun(context.Background(), &model.Run{RunID: "run_failure"})
	if !errors.Is(err, want) {
		t.Fatalf("claim error = %v, want %v", err, want)
	}
}

func TestGenerationEntrypointsRejectDuplicateRunBeforeSideEffects(t *testing.T) {
	tests := []struct {
		name string
		run  func(*Service) error
	}{
		{
			name: "message",
			run: func(service *Service) error {
				_, err := service.SendMessage(context.Background(), SendMessageInput{
					UserID:         7,
					ConversationID: 11,
					Content:        "hello",
					ClientRunID:    "run_duplicate",
					BranchReason:   "default",
				})
				return err
			},
		},
		{
			name: "image",
			run: func(service *Service) error {
				_, err := service.StreamMediaImage(context.Background(), MediaImageInput{
					UserID:            7,
					ConversationID:    11,
					TaskType:          MediaImageTaskGeneration,
					Prompt:            "draw a cat",
					PlatformModelName: "image-model",
					ClientRunID:       "run_duplicate",
					BranchReason:      "default",
				})
				return err
			},
		},
		{
			name: "video",
			run: func(service *Service) error {
				_, err := service.StreamMediaVideo(context.Background(), MediaVideoInput{
					UserID:            7,
					ConversationID:    11,
					TaskType:          MediaVideoTaskGeneration,
					Prompt:            "animate a cat",
					PlatformModelName: "video-model",
					ClientRunID:       "run_duplicate",
					BranchReason:      "default",
				})
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &conversationRunClaimRepositoryStub{
				runs: map[string]model.Run{
					"run_duplicate": {RunID: "run_duplicate", UserID: 7, ConversationID: 11},
				},
				conversation: model.Conversation{ID: 11, UserID: 7, PublicID: "conversation-11", Model: "chat-model"},
			}
			routeResolver := &conversationRunClaimRouteResolver{}
			gateway := &conversationRunClaimLLMGateway{}
			service := &Service{
				cfg:           config.NewRuntime(config.Config{MaxMessageFiles: 10}),
				repo:          repo,
				routeResolver: routeResolver,
				llmClient:     gateway,
				logger:        zap.NewNop(),
			}

			err := test.run(service)
			if !errors.Is(err, ErrDuplicateMessageGenerationRun) {
				t.Fatalf("entrypoint error = %v", err)
			}
			if repo.pairCreateCalls != 0 || repo.branchCreateCalls != 0 {
				t.Fatalf("duplicate run wrote messages: pair=%d branch=%d", repo.pairCreateCalls, repo.branchCreateCalls)
			}
			if routeResolver.resolveCalls != 0 || gateway.generateCalls != 0 {
				t.Fatalf("duplicate run reached route/upstream: route=%d upstream=%d", routeResolver.resolveCalls, gateway.generateCalls)
			}
		})
	}
}

func TestSendMessageEarlyFailureDoesNotPanicBeforeTraceRecorder(t *testing.T) {
	want := errors.New("message persistence failed")
	repo := &conversationRunClaimRepositoryStub{
		runs:          make(map[string]model.Run),
		conversation:  model.Conversation{ID: 11, UserID: 7, PublicID: "conversation-11", Model: "chat-model"},
		pairCreateErr: want,
	}
	service := &Service{
		cfg:    config.NewRuntime(config.Config{MaxMessageFiles: 10}),
		repo:   repo,
		logger: zap.NewNop(),
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("SendMessage panicked before trace recorder initialization: %v", recovered)
		}
	}()
	_, err := service.SendMessage(context.Background(), SendMessageInput{
		UserID:         7,
		ConversationID: 11,
		Content:        "hello",
		ClientRunID:    "run_pair_failure",
		BranchReason:   "default",
	})
	if !errors.Is(err, want) {
		t.Fatalf("SendMessage error = %v, want %v", err, want)
	}
}
