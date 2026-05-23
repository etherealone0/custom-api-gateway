package balancer

import (
	"errors"
	"testing"
)

func TestRoundRobin(t *testing.T) {
	b1, _ := NewBackend("http://b1", 1)
	b2, _ := NewBackend("http://b2", 1)
	b3, _ := NewBackend("http://b3", 1)

	pool := NewServerPool([]*Backend{b1, b2, b3})
	rr := NewRoundRobin(pool)

	// Test 1: Distribution fairness (2-3-1-2-3-1)
	expected := []string{"http://b2", "http://b3", "http://b1", "http://b2", "http://b3", "http://b1"}
	for i, exp := range expected {
		b, err := rr.NextServer()
		if err != nil {
			t.Fatalf("step %d: unexpected error %v", i, err)
		}
		if b.URL.String() != exp {
			t.Errorf("step %d: expected %s, got %s", i, exp, b.URL.String())
		}
	}

	// Test 2: Skip dead backends
	b2.SetAlive(false) // Kill b2
	// Next call: counter=7 -> idx=1 (b2 is dead) -> loop -> counter=8? No, the loop in NextServer doesn't increment the global counter, it uses `i` but the counter is incremented ONCE per backend check!
	// Wait, `idx := rr.counter.Add(1) % uint64(total)` is called IN THE LOOP!
	// So if it checks b2 and b2 is dead, it will increment counter AGAIN in the loop!
	// Let's see:
	// NextServer is called. counter was 6.
	// loop i=0: counter=7, idx=1 (b2). Dead.
	// loop i=1: counter=8, idx=2 (b3). Alive. Returns b3.
	// Next NextServer call:
	// loop i=0: counter=9, idx=0 (b1). Alive. Returns b1.
	// Next NextServer call:
	// loop i=0: counter=10, idx=1 (b2). Dead.
	// loop i=1: counter=11, idx=2 (b3). Alive. Returns b3.
	// So it should be b3, b1, b3, b1.
	expectedAfterKill := []string{"http://b3", "http://b1", "http://b3", "http://b1"}
	for i, exp := range expectedAfterKill {
		b, err := rr.NextServer()
		if err != nil {
			t.Fatalf("step %d after kill: unexpected error %v", i, err)
		}
		if b.URL.String() != exp {
			t.Errorf("step %d after kill: expected %s, got %s", i, exp, b.URL.String())
		}
	}

	// Test 3: All backends dead
	b1.SetAlive(false)
	b3.SetAlive(false)
	_, err := rr.NextServer()
	if !errors.Is(err, ErrNoBackends) {
		t.Errorf("expected ErrNoBackends, got %v", err)
	}

	// Test 4: Single backend
	singleBackend, _ := NewBackend("http://single", 1)
	singlePool := NewServerPool([]*Backend{singleBackend})
	singleRR := NewRoundRobin(singlePool)
	b, err := singleRR.NextServer()
	if err != nil || b.URL.String() != "http://single" {
		t.Errorf("expected http://single, got %v (err: %v)", b, err)
	}
	// And if it dies...
	singleBackend.SetAlive(false)
	_, err = singleRR.NextServer()
	if !errors.Is(err, ErrNoBackends) {
		t.Errorf("expected ErrNoBackends, got %v", err)
	}
}
