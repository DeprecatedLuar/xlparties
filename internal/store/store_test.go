package store

import (
	"path/filepath"
	"testing"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestFriendUpsertAndLookup(t *testing.T) {
	s := openTestStore(t)

	const owner, friend = int64(1001), int64(2002)

	if err := s.UpsertFriend(owner, friend); err != nil {
		t.Fatalf("UpsertFriend: %v", err)
	}

	ids, err := s.FriendIDs(owner)
	if err != nil {
		t.Fatalf("FriendIDs: %v", err)
	}
	if len(ids) != 1 || ids[0] != friend {
		t.Fatalf("FriendIDs = %v, want [%d]", ids, friend)
	}
}

func TestConfigUpsertAndLookup(t *testing.T) {
	s := openTestStore(t)

	if err := s.SetConfig("watch_channel_id", "12345"); err != nil {
		t.Fatalf("SetConfig: %v", err)
	}

	value, ok, err := s.GetConfig("watch_channel_id")
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if !ok || value != "12345" {
		t.Fatalf("GetConfig = (%q, %v), want (\"12345\", true)", value, ok)
	}

	if _, ok, err := s.GetConfig("missing_key"); err != nil {
		t.Fatalf("GetConfig missing: %v", err)
	} else if ok {
		t.Fatal("GetConfig missing key returned ok=true")
	}
}
