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

// buildRewriteOverwrites returns the full overwrite set for a party channel.
// It serves both creation and an ownership handoff or mode change: in every
// mode except public, @everyone is denied, the owner and their friends are
// allowed, then each active friends-of-friends source's own friends folded
// in. In public mode this flips: @everyone is allowed by default instead.
// At creation there are no sources, pending invites, or overrides, so those
// arguments are nil.
//
// In every mode, the owner's globally-blocked users (blockedIDs) are then
// applied as denies, so a block always beats an automatic allow (friend,
// friends-of-friends) regardless of mode. Then each pending /party_invite
// grant is applied, and finally each manual party_overrides row - both are
// deliberate per-channel grants that still win over a global block (an
// owner can /party_allow, or actively invite, a blocked person into one
// channel). party_overrides applies last so it wins over everything above,
// including a pending invite (a ban revokes an outstanding invite too).
//
// sourceIDs are the channel's active friends-of-friends scan sources
// (party_sources); their friend lists are crawled live rather than stored,
// following the "store only what cannot be derived" rule.
//
// alwaysAllowedRoleIDs (from ALWAYS_ALLOWED_ROLES) get a role-level allow on
// every channel in every mode - how a music bot or a staff role gets into an
// otherwise private party. Discord evaluates role allows after the @everyone
// deny, so this grants access in every default-deny mode; it does not protect
// against an owner's /party_block, since member overwrites are evaluated after
// role ones.
func buildRewriteOverwrites(st *store.Store, guildID string, alwaysAllowedRoleIDs []string, botID, ownerID int64, mode string, friendIDs []int64, sourceIDs []int64, pendingInviteIDs []int64, blockedIDs []int64, overrides []store.Override) ([]*discordgo.PermissionOverwrite, error) {
	isPublic := mode == store.AccessModePublic

	allow := make(map[int64]bool, len(friendIDs)+len(sourceIDs)+len(pendingInviteIDs)+len(blockedIDs)+1+len(overrides))
	allow[ownerID] = true
	if !isPublic {
		for _, friendID := range friendIDs {
			allow[friendID] = true
		}
		for _, sourceID := range sourceIDs {
			sourceFriendIDs, err := st.AllowedFriendIDs(sourceID)
			if err != nil {
				return nil, fmt.Errorf("load friends for source %d: %w", sourceID, err)
			}
			for _, friendID := range sourceFriendIDs {
				allow[friendID] = true
			}
		}
	}
	for _, blockedID := range blockedIDs {
		allow[blockedID] = false
	}
	if !isPublic {
		for _, inviteeID := range pendingInviteIDs {
			allow[inviteeID] = true
		}
	}
	for _, o := range overrides {
		allow[o.UserID] = o.Type == "allow"
	}
	// A channel-local @everyone deny is not lifted by guild-level Manage
	// Channels; without this, the bot locks itself out of non-public parties.
	allow[botID] = true

	everyone := &discordgo.PermissionOverwrite{
		ID:   guildID, // @everyone role id equals the guild id
		Type: discordgo.PermissionOverwriteTypeRole,
	}
	if isPublic {
		everyone.Allow = PartyChannelPermissions
	} else {
		everyone.Deny = PartyChannelPermissions
	}

	overwrites := make([]*discordgo.PermissionOverwrite, 0, len(allow)+len(alwaysAllowedRoleIDs)+1)
	overwrites = append(overwrites, everyone)
	for _, roleID := range alwaysAllowedRoleIDs {
		overwrites = append(overwrites, &discordgo.PermissionOverwrite{
			ID:    roleID,
			Type:  discordgo.PermissionOverwriteTypeRole,
			Allow: PartyChannelPermissions,
		})
	}
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
