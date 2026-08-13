package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
)

const (
	turnstileTokenMaxLength = 2048
)

var (
	errTurnstileNotConfigured = errors.New("turnstile is not configured")
	errTurnstileRequired      = errors.New("turnstile verification is required")
	errTurnstileTokenTooLong  = errors.New("turnstile token is too long")
	errTurnstileFailed        = errors.New("turnstile verification failed")
)

type turnstileSiteverifyResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

func (s *Service) verifyRegistrationTurnstile(ctx context.Context, cfg config.Config, tokenValue string, remoteIP string) error {
	if !cfg.TurnstileRegistrationEnabled {
		return nil
	}
	siteKey := strings.TrimSpace(cfg.TurnstileSiteKey)
	if siteKey == "" {
		return nil
	}
	secretKey := strings.TrimSpace(cfg.TurnstileSecretKey)
	if secretKey == "" {
		return errTurnstileNotConfigured
	}

	token := strings.TrimSpace(tokenValue)
	if token == "" {
		return errTurnstileRequired
	}
	if len(token) > turnstileTokenMaxLength {
		return errTurnstileTokenTooLong
	}

	form := url.Values{}
	form.Set("secret", secretKey)
	form.Set("response", token)
	if ip := strings.TrimSpace(remoteIP); ip != "" {
		form.Set("remoteip", ip)
	}

	endpoint := strings.TrimSpace(cfg.TurnstileSiteverifyURL)
	if endpoint == "" {
		endpoint = config.DefaultTurnstileSiteverifyURL
	}
	response, err := s.providerHTTPClient.PostForm(
		ctx,
		endpoint,
		[]string{endpoint},
		form,
		map[string]string{"Accept": "application/json"},
	)
	if err != nil {
		s.warn("turnstile siteverify request failed: " + err.Error())
		return errTurnstileFailed
	}
	var result turnstileSiteverifyResponse
	if err = json.Unmarshal(response.Body, &result); err != nil {
		s.warn(fmt.Sprintf("turnstile siteverify decode failed: status=%d error=%v", response.StatusCode, err))
		return errTurnstileFailed
	}
	if !response.Successful() || !result.Success {
		s.warn(fmt.Sprintf("turnstile siteverify rejected: status=%d error_codes=%s", response.StatusCode, strings.Join(result.ErrorCodes, ",")))
		return errTurnstileFailed
	}
	return nil
}
