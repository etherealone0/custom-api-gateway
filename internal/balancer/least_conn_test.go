package balancer

import (
	"errors"
	"testing"
)

func TestLeastConnections(t *testing.T) {
	b1, _ := NewBackend("http://b1", 1)
	b2, _ := NewBackend("http://b2", 1)
	b3, _ := NewBackend("http://b3", 1)

	pool := NewServerPool([]*Backend{b1, b2, b3})
	lc := NewLeastConnections(pool)

	// Set active connections
	b1.ActiveConnections.Store(10)
	b2.ActiveConnections.Store(5)
	b3.ActiveConnections.Store(20)

	// Should pick b2
	b, err := lc.NextServer()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.URL.String() != "http://b2" {
		t.Errorf("expected http://b2, got %s", b.URL.String())
	}

	// Make b2 busy
	b2.ActiveConnections.Store(25)

	// Should pick b1 now
	b, _ = lc.NextServer()
	if b.URL.String() != "http://b1" {
		t.Errorf("expected http://b1, got %s", b.URL.String())
	}

	// Test skip dead backends
	b1.SetAlive(false) // b1 has lowest connections, but is dead
	// Should pick b3 (20 connections) since b2 has 25
	b, _ = lc.NextServer()
	if b.URL.String() != "http://b3" {
		t.Errorf("expected http://b3, got %s", b.URL.String())
	}

	// Test all dead
	b2.SetAlive(false)
	b3.SetAlive(false)
	_, err = lc.NextServer()
	if !errors.Is(err, ErrNoBackends) {
		t.Errorf("expected ErrNoBackends, got %v", err)
	}
}
