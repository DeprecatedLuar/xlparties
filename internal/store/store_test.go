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

	if err := s.InsertParty(channel, owner, AccessModeFriendsOfFriends); err != nil {
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
	if err := s.InsertParty(channel, owner, AccessModeFriendsOfFriends); err != nil {
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
	if err := s.InsertParty(channel, owner, AccessModeFriendsOfFriends); err != nil {
		t.Fatalf("InsertParty: %v", err)
	}

	for _, mode := range []string{AccessModeFriendsOnly, AccessModeBestiesOnly, AccessModeInviteOnly, AccessModeFriendsOfFriends} {
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
		t.Fatal("UpdateAccessMode with an invalid mode should be rejected by ValidAccessMode")
	}
}

func TestFrenemyFlagsAreIndependent(t *testing.T) {
	s := openTestStore(t)

	const owner, target = int64(1001), int64(2002)

	// Block then friend: both flags should end up set.
	if err := s.UpsertBlock(owner, target); err != nil {
		t.Fatalf("UpsertBlock: %v", err)
	}
	if err := s.UpsertFriend(owner, target); err != nil {
		t.Fatalf("UpsertFriend: %v", err)
	}
	if is, err := s.IsBlocked(owner, target); err != nil || !is {
		t.Fatalf("IsBlocked after block-then-friend = (%v, %v), want (true, nil)", is, err)
	}
	if is, err := s.IsFriend(owner, target); err != nil || !is {
		t.Fatalf("IsFriend after block-then-friend = (%v, %v), want (true, nil)", is, err)
	}

	// Friend-then-block on a fresh pair should leave the same shape.
	const owner2, target2 = int64(3003), int64(4004)
	if err := s.UpsertFriend(owner2, target2); err != nil {
		t.Fatalf("UpsertFriend: %v", err)
	}
	if err := s.UpsertBlock(owner2, target2); err != nil {
		t.Fatalf("UpsertBlock: %v", err)
	}
	if is, err := s.IsFriend(owner2, target2); err != nil || !is {
		t.Fatalf("IsFriend after friend-then-block = (%v, %v), want (true, nil)", is, err)
	}
	if is, err := s.IsBlocked(owner2, target2); err != nil || !is {
		t.Fatalf("IsBlocked after friend-then-block = (%v, %v), want (true, nil)", is, err)
	}

	// AllowedFriendIDs excludes a frenemy; FriendIDs still includes them.
	allowed, err := s.AllowedFriendIDs(owner)
	if err != nil {
		t.Fatalf("AllowedFriendIDs: %v", err)
	}
	if len(allowed) != 0 {
		t.Fatalf("AllowedFriendIDs = %v, want empty (target is a frenemy)", allowed)
	}
	friends, err := s.FriendIDs(owner)
	if err != nil {
		t.Fatalf("FriendIDs: %v", err)
	}
	if len(friends) != 1 || friends[0] != target {
		t.Fatalf("FriendIDs = %v, want [%d]", friends, target)
	}
	frenemies, err := s.FrenemyIDs(owner)
	if err != nil {
		t.Fatalf("FrenemyIDs: %v", err)
	}
	if len(frenemies) != 1 || frenemies[0] != target {
		t.Fatalf("FrenemyIDs = %v, want [%d]", frenemies, target)
	}

	// Removing one flag leaves the other standing.
	if err := s.RemoveBlock(owner, target); err != nil {
		t.Fatalf("RemoveBlock: %v", err)
	}
	if is, err := s.IsBlocked(owner, target); err != nil || is {
		t.Fatalf("IsBlocked after RemoveBlock = (%v, %v), want (false, nil)", is, err)
	}
	if is, err := s.IsFriend(owner, target); err != nil || !is {
		t.Fatalf("IsFriend after RemoveBlock = (%v, %v), want (true, nil)", is, err)
	}

	// Removing both flags deletes the row.
	if err := s.RemoveFriend(owner, target); err != nil {
		t.Fatalf("RemoveFriend: %v", err)
	}
	var rowCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM relationships WHERE granter_id = ? AND grantee_id = ?`, owner, target).Scan(&rowCount); err != nil {
		t.Fatalf("count relationship rows: %v", err)
	}
	if rowCount != 0 {
		t.Fatalf("relationship row still present after clearing both flags, count = %d", rowCount)
	}
}

func TestPresetUpsertReplacesRatherThanDuplicates(t *testing.T) {
	s := openTestStore(t)

	const user = int64(4004)

	if err := s.UpsertPreset(user, AccessModeFriendsOnly); err != nil {
		t.Fatalf("UpsertPreset: %v", err)
	}
	if err := s.UpsertPreset(user, AccessModeInviteOnly); err != nil {
		t.Fatalf("UpsertPreset (replace): %v", err)
	}

	mode, found, err := s.PresetForUser(user)
	if err != nil {
		t.Fatalf("PresetForUser: %v", err)
	}
	if !found || mode != AccessModeInviteOnly {
		t.Fatalf("PresetForUser = (%q, %v), want (%q, true)", mode, found, AccessModeInviteOnly)
	}

	var rowCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM user_presets WHERE user_id = ?`, user).Scan(&rowCount); err != nil {
		t.Fatalf("count preset rows: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("user_presets row count = %d, want 1", rowCount)
	}
}

func TestPresetForUserReportsAbsence(t *testing.T) {
	s := openTestStore(t)

	mode, found, err := s.PresetForUser(5005)
	if err != nil {
		t.Fatalf("PresetForUser: %v", err)
	}
	if found || mode != "" {
		t.Fatalf("PresetForUser = (%q, %v), want (\"\", false)", mode, found)
	}
}

func TestDeletePresetRestoresAbsence(t *testing.T) {
	s := openTestStore(t)

	const user = int64(6006)

	if err := s.UpsertPreset(user, AccessModePublic); err != nil {
		t.Fatalf("UpsertPreset: %v", err)
	}
	if err := s.DeletePreset(user); err != nil {
		t.Fatalf("DeletePreset: %v", err)
	}

	_, found, err := s.PresetForUser(user)
	if err != nil {
		t.Fatalf("PresetForUser: %v", err)
	}
	if found {
		t.Fatalf("PresetForUser found = true after DeletePreset, want false")
	}
}

func TestPresetLimitRoundTripsIndependentlyOfMode(t *testing.T) {
	s := openTestStore(t)

	const user = int64(7007)

	if err := s.UpsertPreset(user, AccessModeFriendsOnly); err != nil {
		t.Fatalf("UpsertPreset: %v", err)
	}
	if err := s.UpsertPresetLimit(user, 5); err != nil {
		t.Fatalf("UpsertPresetLimit: %v", err)
	}

	mode, found, err := s.PresetForUser(user)
	if err != nil {
		t.Fatalf("PresetForUser: %v", err)
	}
	if !found || mode != AccessModeFriendsOnly {
		t.Fatalf("PresetForUser = (%q, %v), want (%q, true)", mode, found, AccessModeFriendsOnly)
	}

	limit, found, err := s.PresetLimitForUser(user)
	if err != nil {
		t.Fatalf("PresetLimitForUser: %v", err)
	}
	if !found || limit != 5 {
		t.Fatalf("PresetLimitForUser = (%d, %v), want (5, true)", limit, found)
	}

	if err := s.UpsertPresetLimit(user, 9); err != nil {
		t.Fatalf("UpsertPresetLimit (replace): %v", err)
	}
	mode, found, err = s.PresetForUser(user)
	if err != nil {
		t.Fatalf("PresetForUser after limit update: %v", err)
	}
	if !found || mode != AccessModeFriendsOnly {
		t.Fatalf("PresetForUser after limit update = (%q, %v), want (%q, true) - mode should be untouched", mode, found, AccessModeFriendsOnly)
	}
}

func TestPresetLimitForUserReportsAbsence(t *testing.T) {
	s := openTestStore(t)

	limit, found, err := s.PresetLimitForUser(8008)
	if err != nil {
		t.Fatalf("PresetLimitForUser: %v", err)
	}
	if found || limit != 0 {
		t.Fatalf("PresetLimitForUser = (%d, %v), want (0, false)", limit, found)
	}
}

func TestPresetLimitOutOfRangeRejected(t *testing.T) {
	s := openTestStore(t)

	if err := s.UpsertPresetLimit(9009, 100); err == nil {
		t.Fatal("UpsertPresetLimit(100) succeeded, want CHECK constraint violation")
	}
	if err := s.UpsertPresetLimit(9009, -1); err == nil {
		t.Fatal("UpsertPresetLimit(-1) succeeded, want CHECK constraint violation")
	}
}

func TestRemoveFriendOnBestieClearsBothFlagsAndPrunes(t *testing.T) {
	s := openTestStore(t)

	const owner, target = int64(1001), int64(2002)

	if err := s.UpsertFriend(owner, target); err != nil {
		t.Fatalf("UpsertFriend: %v", err)
	}
	if err := s.UpsertFavorite(owner, target); err != nil {
		t.Fatalf("UpsertFavorite: %v", err)
	}

	if err := s.RemoveFriend(owner, target); err != nil {
		t.Fatalf("RemoveFriend: %v", err)
	}

	if is, err := s.IsFriend(owner, target); err != nil || is {
		t.Fatalf("IsFriend after RemoveFriend on a bestie = (%v, %v), want (false, nil)", is, err)
	}
	if is, err := s.IsFavorite(owner, target); err != nil || is {
		t.Fatalf("IsFavorite after RemoveFriend on a bestie = (%v, %v), want (false, nil)", is, err)
	}

	var rowCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM relationships WHERE granter_id = ? AND grantee_id = ?`, owner, target).Scan(&rowCount); err != nil {
		t.Fatalf("count relationship rows: %v", err)
	}
	if rowCount != 0 {
		t.Fatalf("relationship row still present after RemoveFriend on a bestie, count = %d", rowCount)
	}
}

func TestFavoritePlusBlockLandsOnlyInBestFrenemyIDs(t *testing.T) {
	s := openTestStore(t)

	const owner, target = int64(1001), int64(2002)

	if err := s.UpsertFriend(owner, target); err != nil {
		t.Fatalf("UpsertFriend: %v", err)
	}
	if err := s.UpsertFavorite(owner, target); err != nil {
		t.Fatalf("UpsertFavorite: %v", err)
	}
	if err := s.UpsertBlock(owner, target); err != nil {
		t.Fatalf("UpsertBlock: %v", err)
	}

	sections := map[string][]int64{}
	var err error
	if sections["Besties"], err = s.AllowedFavoriteIDs(owner); err != nil {
		t.Fatalf("AllowedFavoriteIDs: %v", err)
	}
	if sections["Friends"], err = s.FriendOnlyIDs(owner); err != nil {
		t.Fatalf("FriendOnlyIDs: %v", err)
	}
	if sections["Enemies"], err = s.EnemyIDs(owner); err != nil {
		t.Fatalf("EnemyIDs: %v", err)
	}
	if sections["Frenemies"], err = s.FrenemyIDs(owner); err != nil {
		t.Fatalf("FrenemyIDs: %v", err)
	}
	if sections["Best Frenemies"], err = s.BestFrenemyIDs(owner); err != nil {
		t.Fatalf("BestFrenemyIDs: %v", err)
	}

	for name, ids := range sections {
		present := len(ids) == 1 && ids[0] == target
		want := name == "Best Frenemies"
		if present != want {
			t.Fatalf("section %s contains target = %v, want %v (sections = %v)", name, present, want, sections)
		}
	}
}

func TestListSectionsAreMutuallyExclusive(t *testing.T) {
	s := openTestStore(t)

	const owner = int64(1001)
	const bestie, friend, enemy, frenemy, bestFrenemy = int64(11), int64(12), int64(13), int64(14), int64(15)

	// Bestie: favorite, not blocked.
	if err := s.UpsertFriend(owner, bestie); err != nil {
		t.Fatalf("UpsertFriend(bestie): %v", err)
	}
	if err := s.UpsertFavorite(owner, bestie); err != nil {
		t.Fatalf("UpsertFavorite(bestie): %v", err)
	}

	// Friend: friend, not favorite, not blocked.
	if err := s.UpsertFriend(owner, friend); err != nil {
		t.Fatalf("UpsertFriend(friend): %v", err)
	}

	// Enemy: blocked, not friend.
	if err := s.UpsertBlock(owner, enemy); err != nil {
		t.Fatalf("UpsertBlock(enemy): %v", err)
	}

	// Frenemy: blocked, friend, not favorite.
	if err := s.UpsertFriend(owner, frenemy); err != nil {
		t.Fatalf("UpsertFriend(frenemy): %v", err)
	}
	if err := s.UpsertBlock(owner, frenemy); err != nil {
		t.Fatalf("UpsertBlock(frenemy): %v", err)
	}

	// Best Frenemy: blocked, favorite.
	if err := s.UpsertFriend(owner, bestFrenemy); err != nil {
		t.Fatalf("UpsertFriend(bestFrenemy): %v", err)
	}
	if err := s.UpsertFavorite(owner, bestFrenemy); err != nil {
		t.Fatalf("UpsertFavorite(bestFrenemy): %v", err)
	}
	if err := s.UpsertBlock(owner, bestFrenemy); err != nil {
		t.Fatalf("UpsertBlock(bestFrenemy): %v", err)
	}

	sectionFuncs := map[string]func(int64) ([]int64, error){
		"Besties":        s.AllowedFavoriteIDs,
		"Friends":        s.FriendOnlyIDs,
		"Enemies":        s.EnemyIDs,
		"Frenemies":      s.FrenemyIDs,
		"Best Frenemies": s.BestFrenemyIDs,
	}
	wantSection := map[int64]string{
		bestie:      "Besties",
		friend:      "Friends",
		enemy:       "Enemies",
		frenemy:     "Frenemies",
		bestFrenemy: "Best Frenemies",
	}

	for user, want := range wantSection {
		var found []string
		for name, fn := range sectionFuncs {
			ids, err := fn(owner)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			for _, id := range ids {
				if id == user {
					found = append(found, name)
				}
			}
		}
		if len(found) != 1 || found[0] != want {
			t.Fatalf("user %d found in sections %v, want exactly [%s]", user, found, want)
		}
	}
}
