package llm

import "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"

func newTestClient() *Client {
	return NewClient(security.OutboundPolicy{})
}
