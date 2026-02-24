package store_test

import (
	"path/filepath"
	"testing"

	"github.com/gjed/cie-verona/internal/store"
)

func TestSubscribeAndList(t *testing.T) {
	s := openTestStore(t)
	if err := s.Subscribe(111); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if err := s.Subscribe(222); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	ids, err := s.ListSubscribers()
	if err != nil {
		t.Fatalf("ListSubscribers: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 subscribers, got %d", len(ids))
	}
}

func TestSubscribeIdempotent(t *testing.T) {
	s := openTestStore(t)
	if err := s.Subscribe(111); err != nil {
		t.Fatalf("first Subscribe: %v", err)
	}
	if err := s.Subscribe(111); err != nil {
		t.Fatalf("second Subscribe (idempotent): %v", err)
	}
	ids, _ := s.ListSubscribers()
	if len(ids) != 1 {
		t.Fatalf("expected 1 subscriber, got %d", len(ids))
	}
}

func TestUnsubscribe(t *testing.T) {
	s := openTestStore(t)
	_ = s.Subscribe(111)
	if err := s.Unsubscribe(111); err != nil {
		t.Fatalf("Unsubscribe: %v", err)
	}
	ids, _ := s.ListSubscribers()
	if len(ids) != 0 {
		t.Fatalf("expected 0 subscribers after unsubscribe, got %d", len(ids))
	}
}

func TestUnsubscribeNotSubscribed(t *testing.T) {
	s := openTestStore(t)
	if err := s.Unsubscribe(999); err != nil {
		t.Fatalf("Unsubscribe of unknown id: %v", err)
	}
}

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := store.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
