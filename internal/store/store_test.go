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

func TestIsFriend(t *testing.T) {
	s := openTestStore(t)

	const owner, friend, stranger = int64(1001), int64(2002), int64(3003)

	if is, err := s.IsFriend(owner, friend); err != nil {
		t.Fatalf("IsFriend: %v", err)
	} else if is {
		t.Fatalf("IsFriend(%d,%d) = true before any relationship exists", owner, friend)
	}

	if err := s.UpsertFriend(owner, friend); err != nil {
		t.Fatalf("UpsertFriend: %v", err)
	}

	if is, err := s.IsFriend(owner, friend); err != nil {
		t.Fatalf("IsFriend: %v", err)
	} else if !is {
		t.Fatalf("IsFriend(%d,%d) = false after UpsertFriend", owner, friend)
	}

	if is, err := s.IsFriend(owner, stranger); err != nil {
		t.Fatalf("IsFriend: %v", err)
	} else if is {
		t.Fatalf("IsFriend(%d,%d) = true, want false for unrelated user", owner, stranger)
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

func TestPartySources(t *testing.T) {
	s := openTestStore(t)

	const channel, owner, sourceA, sourceB = int64(9001), int64(1001), int64(2002), int64(3003)

	if err := s.InsertParty(channel, owner); err != nil {
		t.Fatalf("InsertParty: %v", err)
	}

	party, ok, err := s.PartyByChannel(channel)
	if err != nil {
		t.Fatalf("PartyByChannel: %v", err)
	}
	if !ok || party.AccessMode != AccessModeFriendsOfFriends {
		t.Fatalf("PartyByChannel access_mode = %q, want %q", party.AccessMode, AccessModeFriendsOfFriends)
	}

	if err := s.AddSource(channel, sourceA); err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	if err := s.AddSource(channel, sourceB); err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	// Re-adding an existing source must not error (ON CONFLICT DO NOTHING).
	if err := s.AddSource(channel, sourceA); err != nil {
		t.Fatalf("AddSource (duplicate): %v", err)
	}

	ids, err := s.SourceIDsForChannel(channel)
	if err != nil {
		t.Fatalf("SourceIDsForChannel: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("SourceIDsForChannel = %v, want 2 ids", ids)
	}

	if err := s.RemoveSource(channel, sourceA); err != nil {
		t.Fatalf("RemoveSource: %v", err)
	}
	ids, err = s.SourceIDsForChannel(channel)
	if err != nil {
		t.Fatalf("SourceIDsForChannel: %v", err)
	}
	if len(ids) != 1 || ids[0] != sourceB {
		t.Fatalf("SourceIDsForChannel after RemoveSource = %v, want [%d]", ids, sourceB)
	}

	if err := s.RemoveSourcesForChannel(channel); err != nil {
		t.Fatalf("RemoveSourcesForChannel: %v", err)
	}
	ids, err = s.SourceIDsForChannel(channel)
	if err != nil {
		t.Fatalf("SourceIDsForChannel: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("SourceIDsForChannel after RemoveSourcesForChannel = %v, want empty", ids)
	}
}
