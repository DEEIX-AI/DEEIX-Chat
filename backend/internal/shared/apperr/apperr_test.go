package apperr

import (
	"errors"
	"fmt"
	"testing"
)

func TestNewKeepsSentinelIdentityThroughWrapping(t *testing.T) {
	sentinel := New("conversation.not_found", "conversation not found")
	other := New("conversation.not_found", "conversation not found")
	wrapped := fmt.Errorf("load conversation 7: %w", sentinel)

	if !errors.Is(wrapped, sentinel) {
		t.Fatal("wrapped error must still match its sentinel")
	}
	if errors.Is(wrapped, other) {
		t.Fatal("distinct sentinels with equal code must not be confused")
	}
	if sentinel.Error() != "conversation not found" || sentinel.Message() != "conversation not found" {
		t.Fatalf("New must use the message as internal text, got text=%q message=%q", sentinel.Error(), sentinel.Message())
	}
}

func TestFindReadsCodeAndMessageFromChain(t *testing.T) {
	locked := NewMasked("auth.invalid_credentials", "invalid username or password", "account locked")
	wrapped := fmt.Errorf("login user 7: %w", locked)

	found, ok := Find(wrapped)
	if !ok || found != locked {
		t.Fatalf("Find() = %#v, %v; want the masked sentinel", found, ok)
	}
	if found.Code() != "auth.invalid_credentials" || found.Message() != "invalid username or password" {
		t.Fatalf("unexpected contract values code=%q message=%q", found.Code(), found.Message())
	}
	if wrapped.Error() != "login user 7: account locked" {
		t.Fatalf("internal text must stay in the chain, got %q", wrapped.Error())
	}
	if Code(wrapped) != "auth.invalid_credentials" {
		t.Fatalf("Code() = %q", Code(wrapped))
	}
}

func TestFindPrefersOutermostTypedError(t *testing.T) {
	inner := New("file.not_found", "file not found")
	outer := New("knowledge_base.not_ready", "selected knowledge base has no ready files")
	joined := fmt.Errorf("%w: %w", outer, inner)

	if Code(joined) != "knowledge_base.not_ready" {
		t.Fatalf("Code() = %q, want the first typed error in the chain", Code(joined))
	}
}

func TestPlainErrorsAreNotTyped(t *testing.T) {
	if _, ok := Find(errors.New("pq: connection refused")); ok {
		t.Fatal("plain errors must not be treated as typed")
	}
	if _, ok := Find(nil); ok {
		t.Fatal("nil must not be treated as typed")
	}
	if Code(nil) != "" {
		t.Fatalf("Code(nil) = %q", Code(nil))
	}
}

func TestMessageOrUsesTypedMessageAndFallback(t *testing.T) {
	coded := NewMasked("file.not_found", "file not found", "storage lookup failed")
	if got := MessageOr(fmt.Errorf("load file: %w", coded), "fallback"); got != "file not found" {
		t.Fatalf("MessageOr(typed) = %q, want %q", got, "file not found")
	}
	if got := MessageOr(errors.New("pq: connection refused"), "fallback"); got != "fallback" {
		t.Fatalf("MessageOr(plain) = %q, want fallback", got)
	}
}

func TestNewRejectsEmptyContract(t *testing.T) {
	for name, fn := range map[string]func(){
		"empty code":    func() { New("", "message") },
		"empty message": func() { New("code", "") },
		"empty text":    func() { NewMasked("code", "message", "") },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("expected panic for incomplete error contract")
				}
			}()
			fn()
		})
	}
}
