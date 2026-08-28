package lifecycle

import "testing"

func TestShutdownDrainLifecycle(t *testing.T) {
	s := NewShutdown()
	if s.Draining() {
		t.Fatal("new shutdown must not be draining")
	}
	select {
	case <-s.Done():
		t.Fatal("done channel must block before BeginDrain")
	default:
	}

	s.BeginDrain()
	s.BeginDrain() // 幂等：重复触发不应 panic。
	if !s.Draining() {
		t.Fatal("expected draining after BeginDrain")
	}
	select {
	case <-s.Done():
	default:
		t.Fatal("done channel must be closed after BeginDrain")
	}
}

func TestShutdownNilReceiverIsSafe(t *testing.T) {
	var s *Shutdown
	s.BeginDrain()
	if s.Draining() {
		t.Fatal("nil shutdown must not report draining")
	}
	select {
	case <-s.Done():
		t.Fatal("nil shutdown done channel must never be ready")
	default:
	}
}
