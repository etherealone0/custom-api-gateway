package balancer

import (
	"errors"
	"testing"
)

func TestWeightedRoundRobin(t *testing.T) {
	// Weights: b1=5, b2=1, b3=1
	// Total weight = 7. Sequence should have 5 of b1, 1 of b2, 1 of b3 per 7 requests
	b1, _ := NewBackend("http://b1", 5)
	b2, _ := NewBackend("http://b2", 1)
	b3, _ := NewBackend("http://b3", 1)

	pool := NewServerPool([]*Backend{b1, b2, b3})
	wrr := NewWeightedRoundRobin(pool)

	counts := make(map[string]int)
	for i := 0; i < 7; i++ {
		b, err := wrr.NextServer()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		counts[b.URL.String()]++
	}

	if counts["http://b1"] != 5 {
		t.Errorf("expected 5 hits for b1, got %d", counts["http://b1"])
	}
	if counts["http://b2"] != 1 {
		t.Errorf("expected 1 hit for b2, got %d", counts["http://b2"])
	}
	if counts["http://b3"] != 1 {
		t.Errorf("expected 1 hit for b3, got %d", counts["http://b3"])
	}

	// Test 2: Skip dead backends
	b1.SetAlive(false) // Kill the heaviest backend
	// Now only b2 (wt 1) and b3 (wt 1) are alive.
	// Expected sequence is b2, b3, b2, b3...
	b, _ := wrr.NextServer()
	if b.URL.String() == "http://b1" {
		t.Errorf("expected b1 to be skipped")
	}

	// Test 3: All dead
	b2.SetAlive(false)
	b3.SetAlive(false)
	_, err := wrr.NextServer()
	if !errors.Is(err, ErrNoBackends) {
		t.Errorf("expected ErrNoBackends, got %v", err)
	}

	// Test 4: Dynamic pool resize
	newB1, _ := NewBackend("http://new1", 1)
	pool.SetBackends([]*Backend{newB1})
	b, err = wrr.NextServer()
	if err != nil || b.URL.String() != "http://new1" {
		t.Errorf("expected http://new1, got %v (err: %v)", b, err)
	}
}
