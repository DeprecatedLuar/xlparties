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

	channels, err := s.ChannelsForSource(sourceB)
	if err != nil {
		t.Fatalf("ChannelsForSource: %v", err)
	}
	if len(channels) != 1 || channels[0] != channel {
		t.Fatalf("ChannelsForSource(%d) = %v, want [%d]", sourceB, channels, channel)
	}

	if err := s.RemoveSourcesForChannel(channel); err != nil {
		t.Fatalf("RemoveSourcesForChannel: %v", err)
	}
	channels, err = s.ChannelsForSource(sourceB)
	if err != nil {
		t.Fatalf("ChannelsForSource: %v", err)
	}
	if len(channels) != 0 {
		t.Fatalf("ChannelsForSource after RemoveSourcesForChannel = %v, want empty", channels)
	}
	ids, err = s.SourceIDsForChannel(channel)
	if err != nil {
		t.Fatalf("SourceIDsForChannel: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("SourceIDsForChannel after RemoveSourcesForChannel = %v, want empty", ids)
	}
}

func TestIsBlocked(t *testing.T) {
	s := openTestStore(t)

	const owner, blocked, stranger = int64(1001), int64(2002), int64(3003)

	if is, err := s.IsBlocked(owner, blocked); err != nil {
		t.Fatalf("IsBlocked: %v", err)
	} else if is {
		t.Fatalf("IsBlocked(%d,%d) = true before any relationship exists", owner, blocked)
	}

	if err := s.UpsertBlock(owner, blocked); err != nil {
		t.Fatalf("UpsertBlock: %v", err)
	}

	if is, err := s.IsBlocked(owner, blocked); err != nil {
		t.Fatalf("IsBlocked: %v", err)
	} else if !is {
		t.Fatalf("IsBlocked(%d,%d) = false after UpsertBlock", owner, blocked)
	}

	if is, err := s.IsBlocked(owner, stranger); err != nil {
		t.Fatalf("IsBlocked: %v", err)
	} else if is {
		t.Fatalf("IsBlocked(%d,%d) = true, want false for unrelated user", owner, stranger)
	}
}

func TestPartyInvites(t *testing.T) {
	s := openTestStore(t)

	const channel, owner, invitee, otherInvitee = int64(9001), int64(1001), int64(4001), int64(4002)
	if err := s.InsertParty(channel, owner); err != nil {
		t.Fatalf("InsertParty: %v", err)
	}

	if err := s.AddInvite(channel, invitee, 1000); err != nil {
		t.Fatalf("AddInvite: %v", err)
	}
	if err := s.AddInvite(channel, otherInvitee, 2000); err != nil {
		t.Fatalf("AddInvite: %v", err)
	}

	ids, err := s.InviteIDsForChannel(channel)
	if err != nil {
		t.Fatalf("InviteIDsForChannel: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("InviteIDsForChannel = %v, want 2 ids", ids)
	}

	// Re-inviting an existing pending invite refreshes expires_at rather
	// than erroring or duplicating the row.
	if err := s.AddInvite(channel, invitee, 5000); err != nil {
		t.Fatalf("AddInvite (refresh): %v", err)
	}
	all, err := s.AllInvites()
	if err != nil {
		t.Fatalf("AllInvites: %v", err)
	}
	var found bool
	for _, inv := range all {
		if inv.ChannelID == channel && inv.UserID == invitee {
			found = true
			if inv.ExpiresAt != 5000 {
				t.Fatalf("invite expires_at = %d, want 5000 after refresh", inv.ExpiresAt)
			}
		}
	}
	if !found {
		t.Fatalf("AllInvites = %v, missing invite for (%d,%d)", all, channel, invitee)
	}

	if err := s.RemoveInvite(channel, invitee); err != nil {
		t.Fatalf("RemoveInvite: %v", err)
	}
	ids, err = s.InviteIDsForChannel(channel)
	if err != nil {
		t.Fatalf("InviteIDsForChannel: %v", err)
	}
	if len(ids) != 1 || ids[0] != otherInvitee {
		t.Fatalf("InviteIDsForChannel after RemoveInvite = %v, want [%d]", ids, otherInvitee)
	}

	if err := s.RemoveInvitesForChannel(channel); err != nil {
		t.Fatalf("RemoveInvitesForChannel: %v", err)
	}
	ids, err = s.InviteIDsForChannel(channel)
	if err != nil {
		t.Fatalf("InviteIDsForChannel: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("InviteIDsForChannel after RemoveInvitesForChannel = %v, want empty", ids)
	}
}

func TestUpdateAccessMode(t *testing.T) {
	s := openTestStore(t)

	const channel, owner = int64(9001), int64(1001)
	if err := s.InsertParty(channel, owner); err != nil {
		t.Fatalf("InsertParty: %v", err)
	}

	for _, mode := range []string{AccessModeFriendsOnly, AccessModeInviteOnly, AccessModeFriendsOfFriends} {
		if err := s.UpdateAccessMode(channel, mode); err != nil {
			t.Fatalf("UpdateAccessMode(%q): %v", mode, err)
		}
		party, ok, err := s.PartyByChannel(channel)
		if err != nil {
			t.Fatalf("PartyByChannel: %v", err)
		}
		if !ok || party.AccessMode != mode {
			t.Fatalf("access_mode after UpdateAccessMode(%q) = %q", mode, party.AccessMode)
		}
	}

	if err := s.UpdateAccessMode(channel, "not_a_real_mode"); err == nil {
		t.Fatal("UpdateAccessMode with an invalid mode should fail the access_mode CHECK constraint")
	}
}
