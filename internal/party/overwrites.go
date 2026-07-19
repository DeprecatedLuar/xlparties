package party

import (
	"fmt"
	"strconv"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/store"
)

// PartyChannelPermissions is the pair of permissions the whole overwrite
// model turns on or off: seeing and joining the channel. Exported because
// the party_allow/party_block commands write the same pair to a single overwrite.
const PartyChannelPermissions = discordgo.PermissionViewChannel | discordgo.PermissionVoiceConnect

// buildCreationOverwrites returns the full overwrite set for a new party
// channel per spec.md Creation: @everyone denied, the owner allowed, and
// each of the owner's friends allowed.
func buildCreationOverwrites(guildID string, ownerID int64, friendIDs []int64) []*discordgo.PermissionOverwrite {
	overwrites := make([]*discordgo.PermissionOverwrite, 0, len(friendIDs)+2)

	overwrites = append(overwrites, &discordgo.PermissionOverwrite{
		ID:   guildID, // @everyone role id equals the guild id
		Type: discordgo.PermissionOverwriteTypeRole,
		Deny: PartyChannelPermissions,
	})

	overwrites = append(overwrites, memberOverwrite(ownerID, true))
	for _, friendID := range friendIDs {
		overwrites = append(overwrites, memberOverwrite(friendID, true))
	}

	return overwrites
}

// buildRewriteOverwrites returns the full overwrite set for a party channel
// after an ownership handoff or mode change, per spec.md Ownership Rewrite:
// in every mode except public, @everyone is denied, the new owner and their
// friends are allowed, then each active friends-of-friends source's own
// friends folded in, then each pending /party_invite grant, then each
// manual party_overrides row applied last so it wins over every default
// (including a pending invite - a ban revokes an outstanding invite too).
//
// In public mode this flips: @everyone is allowed by default, and the
// owner's globally-blocked users (blockedIDs) are the auto-deny set instead
// - the mirror image of friends being the auto-allow set elsewhere.
// party_overrides still applies last and still wins, so an owner can
// /party_allow a globally-blocked person into this one channel.
//
// sourceIDs are the channel's active friends-of-friends scan sources
// (party_sources); their friend lists are crawled live rather than stored,
// per spec.md's "store what cannot be derived" rule.
func buildRewriteOverwrites(st *store.Store, guildID string, ownerID int64, mode string, friendIDs []int64, sourceIDs []int64, pendingInviteIDs []int64, blockedIDs []int64, overrides []store.Override) ([]*discordgo.PermissionOverwrite, error) {
	isPublic := mode == store.AccessModePublic

	allow := make(map[int64]bool, len(friendIDs)+len(sourceIDs)+len(pendingInviteIDs)+len(blockedIDs)+1+len(overrides))
	allow[ownerID] = true
	if !isPublic {
		for _, friendID := range friendIDs {
			allow[friendID] = true
		}
		for _, sourceID := range sourceIDs {
			sourceFriendIDs, err := st.FriendIDs(sourceID)
			if err != nil {
				return nil, fmt.Errorf("load friends for source %d: %w", sourceID, err)
			}
			for _, friendID := range sourceFriendIDs {
				allow[friendID] = true
			}
		}
		for _, inviteeID := range pendingInviteIDs {
			allow[inviteeID] = true
		}
	} else {
		for _, blockedID := range blockedIDs {
			allow[blockedID] = false
		}
	}
	for _, o := range overrides {
		allow[o.UserID] = o.Type == "allow"
	}

	everyone := &discordgo.PermissionOverwrite{
		ID:   guildID, // @everyone role id equals the guild id
		Type: discordgo.PermissionOverwriteTypeRole,
	}
	if isPublic {
		everyone.Allow = PartyChannelPermissions
	} else {
		everyone.Deny = PartyChannelPermissions
	}

	overwrites := make([]*discordgo.PermissionOverwrite, 0, len(allow)+1)
	overwrites = append(overwrites, everyone)
	for userID, allowed := range allow {
		overwrites = append(overwrites, memberOverwrite(userID, allowed))
	}
	return overwrites, nil
}

func memberOverwrite(userID int64, allow bool) *discordgo.PermissionOverwrite {
	ow := &discordgo.PermissionOverwrite{
		ID:   strconv.FormatInt(userID, 10),
		Type: discordgo.PermissionOverwriteTypeMember,
	}
	if allow {
		ow.Allow = PartyChannelPermissions
	} else {
		ow.Deny = PartyChannelPermissions
	}
	return ow
}
