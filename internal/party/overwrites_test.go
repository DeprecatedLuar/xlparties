package party

import (
	"path/filepath"
	"testing"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/store"
)

func openTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func allowedIDs(t *testing.T, overwrites []*discordgo.PermissionOverwrite) map[string]bool {
	t.Helper()
	allowed := make(map[string]bool)
	for _, ow := range overwrites {
		if ow.Type == discordgo.PermissionOverwriteTypeMember && ow.Allow&PartyChannelPermissions == PartyChannelPermissions {
			allowed[ow.ID] = true
		}
	}
	return allowed
}

func TestBuildRewriteOverwritesCrawlsSourceFriends(t *testing.T) {
	s := openTestStore(t)

	const guildID = "1"
	const owner, ownerFriend = int64(1001), int64(1002)
	const source, sourceFriend = int64(2001), int64(2002)

	if err := s.UpsertFriend(owner, ownerFriend); err != nil {
		t.Fatalf("UpsertFriend: %v", err)
	}
	if err := s.UpsertFriend(source, sourceFriend); err != nil {
		t.Fatalf("UpsertFriend: %v", err)
	}

	ownerFriendIDs, err := s.FriendIDs(owner)
	if err != nil {
		t.Fatalf("FriendIDs: %v", err)
	}

	overwrites, err := buildRewriteOverwrites(s, guildID, owner, store.AccessModeFriendsOfFriends, ownerFriendIDs, []int64{source}, nil, nil, nil)
	if err != nil {
		t.Fatalf("buildRewriteOverwrites: %v", err)
	}

	allowed := allowedIDs(t, overwrites)
	for _, wantAllowed := range []int64{owner, ownerFriend, sourceFriend} {
		if id := formatID(wantAllowed); !allowed[id] {
			t.Errorf("expected %d to be allowed, allowed set = %v", wantAllowed, allowed)
		}
	}
	// The source itself is not a friend of the owner and was not passed in
	// friendIDs, so it must not appear as an allow entry from this crawl.
	if allowed[formatID(source)] {
		t.Errorf("source %d should not itself be allowed unless also a friend", source)
	}
}

func TestBuildRewriteOverwritesOverrideWinsOverSourceFriend(t *testing.T) {
	s := openTestStore(t)

	const guildID = "1"
	const owner = int64(1001)
	const source, sourceFriend = int64(2001), int64(2002)

	if err := s.UpsertFriend(source, sourceFriend); err != nil {
		t.Fatalf("UpsertFriend: %v", err)
	}

	overrides := []store.Override{{ChannelID: 1, UserID: sourceFriend, Type: "deny"}}
	overwrites, err := buildRewriteOverwrites(s, guildID, owner, store.AccessModeFriendsOfFriends, nil, []int64{source}, nil, nil, overrides)
	if err != nil {
		t.Fatalf("buildRewriteOverwrites: %v", err)
	}

	for _, ow := range overwrites {
		if ow.Type == discordgo.PermissionOverwriteTypeMember && ow.ID == formatID(sourceFriend) {
			if ow.Deny&PartyChannelPermissions != PartyChannelPermissions {
				t.Fatalf("expected party_block override to win over source-friend grant, got overwrite %+v", ow)
			}
			return
		}
	}
	t.Fatalf("no overwrite found for denied source-friend %d", sourceFriend)
}

func TestBuildRewriteOverwritesPendingInviteSurvivesRebuild(t *testing.T) {
	s := openTestStore(t)

	const guildID = "1"
	const owner, invitee = int64(1001), int64(3001)

	overwrites, err := buildRewriteOverwrites(s, guildID, owner, store.AccessModeFriendsOfFriends, nil, nil, []int64{invitee}, nil, nil)
	if err != nil {
		t.Fatalf("buildRewriteOverwrites: %v", err)
	}

	allowed := allowedIDs(t, overwrites)
	if !allowed[formatID(invitee)] {
		t.Errorf("expected pending invitee %d to be allowed, allowed set = %v", invitee, allowed)
	}
}

func TestBuildRewriteOverwritesDenyOverrideWinsOverPendingInvite(t *testing.T) {
	s := openTestStore(t)

	const guildID = "1"
	const owner, invitee = int64(1001), int64(3001)

	overrides := []store.Override{{ChannelID: 1, UserID: invitee, Type: "deny"}}
	overwrites, err := buildRewriteOverwrites(s, guildID, owner, store.AccessModeFriendsOfFriends, nil, nil, []int64{invitee}, nil, overrides)
	if err != nil {
		t.Fatalf("buildRewriteOverwrites: %v", err)
	}

	for _, ow := range overwrites {
		if ow.Type == discordgo.PermissionOverwriteTypeMember && ow.ID == formatID(invitee) {
			if ow.Deny&PartyChannelPermissions != PartyChannelPermissions {
				t.Fatalf("expected party_block override to win over pending invite, got overwrite %+v", ow)
			}
			return
		}
	}
	t.Fatalf("no overwrite found for denied invitee %d", invitee)
}

func TestBuildRewriteOverwritesPublicModeDeniesBlockedAllowsEveryoneElse(t *testing.T) {
	s := openTestStore(t)

	const guildID = "1"
	const owner, blocked, allowOverridden = int64(1001), int64(4001), int64(4002)

	overrides := []store.Override{{ChannelID: 1, UserID: allowOverridden, Type: "allow"}}
	overwrites, err := buildRewriteOverwrites(s, guildID, owner, store.AccessModePublic, nil, nil, nil, []int64{blocked, allowOverridden}, overrides)
	if err != nil {
		t.Fatalf("buildRewriteOverwrites: %v", err)
	}

	var everyone *discordgo.PermissionOverwrite
	memberOverwrites := make(map[string]*discordgo.PermissionOverwrite)
	for _, ow := range overwrites {
		if ow.Type == discordgo.PermissionOverwriteTypeRole && ow.ID == guildID {
			everyone = ow
		}
		if ow.Type == discordgo.PermissionOverwriteTypeMember {
			memberOverwrites[ow.ID] = ow
		}
	}

	if everyone == nil || everyone.Allow&PartyChannelPermissions != PartyChannelPermissions {
		t.Fatalf("expected @everyone to be allowed in public mode, got %+v", everyone)
	}

	if ow := memberOverwrites[formatID(blocked)]; ow == nil || ow.Deny&PartyChannelPermissions != PartyChannelPermissions {
		t.Errorf("expected blocked user %d to have a deny overwrite, got %+v", blocked, ow)
	}

	// A manual /party_allow overrides the auto-deny from the block list.
	if ow := memberOverwrites[formatID(allowOverridden)]; ow == nil || ow.Allow&PartyChannelPermissions != PartyChannelPermissions {
		t.Errorf("expected manually-allowed blocked user %d to be allowed, got %+v", allowOverridden, ow)
	}
}

func formatID(id int64) string {
	return memberOverwrite(id, true).ID
}
