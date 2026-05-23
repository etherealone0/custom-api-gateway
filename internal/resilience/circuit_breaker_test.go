package resilience

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestCircuitBreaker_StateTransitions(t *testing.T) {
	cb := NewCircuitBreaker("test-backend", 3, 2, 50*time.Millisecond)

	// 1. Closed state -> Allow returns nil
	if err := cb.Allow(); err != nil {
		t.Fatalf("expected no error in Closed state, got %v", err)
	}

	// 2. Closed -> Open on failure threshold
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.GetState() != StateClosed {
		t.Fatalf("expected Closed state, got %v", cb.GetState())
	}

	cb.RecordFailure() // 3rd failure hits threshold
	if cb.GetState() != StateOpen {
		t.Fatalf("expected Open state, got %v", cb.GetState())
	}
	if err := cb.Allow(); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("expected ErrCircuitOpen in Open state, got %v", err)
	}

	// 3. Open -> Half-Open on timeout
	time.Sleep(60 * time.Millisecond) // Wait for timeout
	if err := cb.Allow(); err != nil {
		t.Fatalf("expected no error (Half-Open transition), got %v", err)
	}
	if cb.GetState() != StateHalfOpen {
		t.Fatalf("expected Half-Open state, got %v", cb.GetState())
	}

	// 4. Half-Open -> Open on single failure
	cb.RecordFailure()
	if cb.GetState() != StateOpen {
		t.Fatalf("expected Open state after failure in Half-Open, got %v", cb.GetState())
	}

	// 5. Open -> Half-Open -> Closed on successes
	time.Sleep(60 * time.Millisecond) // Wait for timeout again
	cb.Allow()                        // Transitions to Half-Open
	cb.RecordSuccess()
	if cb.GetState() != StateHalfOpen {
		t.Fatalf("expected Half-Open state after 1 success (need 2), got %v", cb.GetState())
	}
	cb.RecordSuccess()
	if cb.GetState() != StateClosed {
		t.Fatalf("expected Closed state after 2 successes, got %v", cb.GetState())
	}
}

func TestCircuitBreaker_Concurrency(t *testing.T) {
	cb := NewCircuitBreaker("test-backend", 50, 1, 100*time.Millisecond)

	var wg sync.WaitGroup
	// Spawn 100 goroutines to concurrently record failures
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cb.RecordFailure()
		}()
	}
	wg.Wait()

	if cb.GetState() != StateOpen {
		t.Fatalf("expected Open state after concurrent failures, got %v", cb.GetState())
	}
}
