package conversation

import (
	"context"
	"errors"
	"net"
	"strings"
	"time"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/textutil"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/traceid"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/background"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
	"go.uber.org/zap"
)

const MessageErrorCodeMediaArtifactUnavailable = "media.artifact_unavailable"

const generatedMediaArtifactFinalizeTimeout = 5 * time.Second

// generatedMediaArtifactError 将安全的用户语义与仅供内部诊断的原始原因隔离。
// 错误链只暴露安全的应用层哨兵，底层技术原因仅保留在 cause 中供结构化诊断。
type generatedMediaArtifactError struct {
	mediaType string
	stage     string
	cause     error
}

func (e *generatedMediaArtifactError) Error() string {
	return ErrGeneratedMediaArtifactUnavailable.Error()
}

func (e *generatedMediaArtifactError) Unwrap() error {
	return ErrGeneratedMediaArtifactUnavailable
}

// newGeneratedMediaArtifactError 收敛媒体制品技术错误，同时保留结构化诊断所需的内部原因。
func newGeneratedMediaArtifactError(mediaType string, stage string, cause error) error {
	return &generatedMediaArtifactError{
		mediaType: strings.TrimSpace(mediaType),
		stage:     strings.TrimSpace(stage),
		cause:     cause,
	}
}

// finalizeGeneratedMediaArtifactFailure 统一收敛制品读取失败的运行状态、持久化状态和诊断日志。
// 用户主动取消优先于技术错误，避免把 canceled run 重新记录为制品故障。
func (s *Service) finalizeGeneratedMediaArtifactFailure(
	ctx context.Context,
	run *model.Run,
	assistantMessageID uint,
	artifactIndex int,
	artifactCount int,
	err error,
) error {
	if err == nil {
		return nil
	}
	runID := ""
	if run != nil {
		runID = run.RunID
	}
	status := "error"
	persistCtx := ctx
	finalErr := err
	if s.isCanceledMediaGeneration(ctx, runID, err) {
		status = "canceled"
		var cancel context.CancelFunc
		persistCtx, cancel = background.WithTimeout(ctx, generatedMediaArtifactFinalizeTimeout)
		defer cancel()
		finalErr = ErrMessageGenerationCanceled
	} else if errors.Is(err, ErrGeneratedMediaArtifactUnavailable) {
		s.logGeneratedMediaArtifactFailure(ctx, run, artifactIndex, artifactCount, err)
	}
	if s != nil && s.repo != nil && assistantMessageID != 0 {
		_ = s.repo.UpdateMessageState(
			persistCtx,
			assistantMessageID,
			status,
			classifyRunErrorCode(finalErr),
			textutil.TruncateTrimmed(messageErrorSummary(finalErr), 255),
		)
	}
	return finalErr
}

// logGeneratedMediaArtifactFailure 在拥有完整运行上下文的调用点记录一次诊断日志。
// 不记录制品 URL、凭据、响应正文或媒体内容，避免签名参数和用户数据进入日志。
func (s *Service) logGeneratedMediaArtifactFailure(
	ctx context.Context,
	run *model.Run,
	artifactIndex int,
	artifactCount int,
	err error,
) {
	if s == nil || s.logger == nil || run == nil || err == nil {
		return
	}
	details := generatedMediaArtifactFailureDetails(err)
	s.logger.Warn("generated_media_artifact_failed",
		zap.String("trace_id", traceid.FromContext(ctx)),
		zap.String("request_id", strings.TrimSpace(run.RequestID)),
		zap.String("run_id", strings.TrimSpace(run.RunID)),
		zap.String("error_code", MessageErrorCodeMediaArtifactUnavailable),
		zap.Uint("user_id", run.UserID),
		zap.Uint("conversation_id", run.ConversationID),
		zap.String("task_type", strings.TrimSpace(run.TaskType)),
		zap.String("endpoint", strings.TrimSpace(run.Endpoint)),
		zap.String("media_type", details.mediaType),
		zap.Int("artifact_index", artifactIndex),
		zap.Int("artifact_count", artifactCount),
		zap.String("failure_stage", details.stage),
		zap.String("failure_class", classifyGeneratedMediaArtifactFailure(details.stage, details.cause)),
		zap.Uint("upstream_id", run.UpstreamID),
		zap.Uint("upstream_model_id", run.UpstreamModelID),
		zap.String("upstream_name", strings.TrimSpace(run.UpstreamName)),
		zap.String("provider_protocol", strings.TrimSpace(run.ProviderProtocol)),
		zap.String("platform_model_name", strings.TrimSpace(run.PlatformModelName)),
		zap.String("upstream_model_name", strings.TrimSpace(run.UpstreamModelName)),
		zap.String("routed_binding_code", strings.TrimSpace(run.RoutedBindingCode)),
		zap.String("model_vendor", strings.TrimSpace(run.ModelVendor)),
		zap.Error(details.cause),
	)
}

type generatedMediaArtifactFailure struct {
	mediaType string
	stage     string
	cause     error
}

func generatedMediaArtifactFailureDetails(err error) generatedMediaArtifactFailure {
	var artifactErr *generatedMediaArtifactError
	if errors.As(err, &artifactErr) && artifactErr != nil {
		return generatedMediaArtifactFailure{
			mediaType: artifactErr.mediaType,
			stage:     artifactErr.stage,
			cause:     artifactErr.cause,
		}
	}
	return generatedMediaArtifactFailure{cause: err}
}

func classifyGeneratedMediaArtifactFailure(stage string, cause error) string {
	switch {
	case errors.Is(cause, security.ErrUnsafeOutboundURL):
		return "outbound_policy"
	case errors.Is(cause, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(cause, context.Canceled):
		return "canceled"
	}
	var networkError net.Error
	if errors.As(cause, &networkError) && networkError.Timeout() {
		return "timeout"
	}
	switch strings.TrimSpace(stage) {
	case "decode", "validation":
		return "invalid_artifact"
	case "configuration":
		return "configuration"
	default:
		return "upstream_download"
	}
}
