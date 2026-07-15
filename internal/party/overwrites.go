package party

import (
	"strconv"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/store"
)

// PartyChannelPermissions is the pair of permissions the whole overwrite
// model turns on or off: seeing and joining the channel. Exported because
// the vc_allow/vc_deny commands write the same pair to a single overwrite.
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
// after an ownership handoff, per spec.md Ownership Rewrite: @everyone
// denied, the new owner and their friends allowed, then each manual
// party_overrides row applied last so it wins over the friend defaults.
func buildRewriteOverwrites(guildID string, ownerID int64, friendIDs []int64, overrides []store.Override) []*discordgo.PermissionOverwrite {
	allow := make(map[int64]bool, len(friendIDs)+1+len(overrides))
	allow[ownerID] = true
	for _, friendID := range friendIDs {
		allow[friendID] = true
	}
	for _, o := range overrides {
		allow[o.UserID] = o.Type == "allow"
	}

	overwrites := make([]*discordgo.PermissionOverwrite, 0, len(allow)+1)
	overwrites = append(overwrites, &discordgo.PermissionOverwrite{
		ID:   guildID, // @everyone role id equals the guild id
		Type: discordgo.PermissionOverwriteTypeRole,
		Deny: PartyChannelPermissions,
	})
	for userID, allowed := range allow {
		overwrites = append(overwrites, memberOverwrite(userID, allowed))
	}
	return overwrites
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
