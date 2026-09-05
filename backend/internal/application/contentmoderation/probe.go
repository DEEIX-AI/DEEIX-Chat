package contentmoderation

import (
	"context"
	"encoding/base64"
	"errors"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/textutil"
	"strings"
	"time"

	"go.uber.org/zap"
)

// ProbeResult is the super-admin probe response for one modality.
type ProbeResult struct {
	Valid   bool
	Model   string
	Latency int64
	Error   string
}

// ProbeResponse covers text and image probes.
type ProbeResponse struct {
	Text  ProbeResult
	Image ProbeResult
}

// 1x1 transparent PNG.
var probePNG, _ = base64.StdEncoding.DecodeString(
	"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
)

// Probe validates the saved config against built-in harmless samples.
func (s *Service) Probe(ctx context.Context, actorRole string) (*ProbeResponse, error) {
	if !isSuperAdmin(actorRole) {
		return nil, ErrSuperAdminRequired
	}
	cfg, err := s.readRuntimeConfig(ctx)
	if err != nil {
		return nil, err
	}
	out := &ProbeResponse{}
	if s.provider == nil {
		return nil, ErrModerationService
	}
	providerConfig := providerConfigFromRuntime(cfg)

	// Text probe
	{
		started := time.Now()
		resp, err := s.provider.ModerateText(ctx, providerConfig, "hello", nil, ModalityText)
		out.Text.Latency = time.Since(started).Milliseconds()
		if err != nil {
			s.logWarn("content_moderation_probe_failed", zap.String("modality", ModalityText), zap.Error(err))
			out.Text.Error = probeErrorMessage(err)
		} else if resp == nil || len(resp.Results) == 0 || resp.Results[0].Categories == nil {
			out.Text.Error = probeErrorMessage(ErrModerationInvalidResp)
		} else {
			out.Text.Valid = true
			out.Text.Model = textutil.FirstNonEmpty(resp.Model, cfg.Model)
		}
	}

	// Image probe
	{
		started := time.Now()
		resp, err := s.provider.ModerateImages(ctx, providerConfig, []ProviderImage{{Data: probePNG, MimeType: "image/png"}}, nil, ModalityImage)
		out.Image.Latency = time.Since(started).Milliseconds()
		if err != nil {
			s.logWarn("content_moderation_probe_failed", zap.String("modality", ModalityImage), zap.Error(err))
			out.Image.Error = probeErrorMessage(err)
		} else if resp == nil || len(resp.Results) == 0 || resp.Results[0].Categories == nil {
			out.Image.Error = probeErrorMessage(ErrModerationInvalidResp)
		} else {
			// Official Omni responses include category_applied_input_types; require image proof.
			applied := resp.Results[0].CategoryAppliedInputTypes
			foundImage := false
			for _, types := range applied {
				for _, t := range types {
					if strings.EqualFold(strings.TrimSpace(t), "image") {
						foundImage = true
						break
					}
				}
				if foundImage {
					break
				}
			}
			if !foundImage {
				out.Image.Valid = false
				out.Image.Error = "moderation response missing image category_applied_input_types"
			} else {
				out.Image.Valid = true
				out.Image.Model = textutil.FirstNonEmpty(resp.Model, cfg.Model)
			}
		}
	}
	return out, nil
}

func probeErrorMessage(err error) string {
	switch {
	case errors.Is(err, ErrModerationTimeout):
		return ErrModerationTimeout.Error()
	case errors.Is(err, ErrModerationRateLimited):
		return ErrModerationRateLimited.Error()
	case errors.Is(err, ErrModerationInvalidResp):
		return ErrModerationInvalidResp.Error()
	case errors.Is(err, ErrModerationNetwork):
		return ErrModerationNetwork.Error()
	case errors.Is(err, ErrModerationService):
		return ErrModerationService.Error()
	default:
		return ErrProbeFailed.Error()
	}
}
