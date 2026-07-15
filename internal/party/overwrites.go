package party

import (
	"strconv"

	"github.com/bwmarrin/discordgo"
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

	overwrites = append(overwrites, memberAllowOverwrite(ownerID))
	for _, friendID := range friendIDs {
		overwrites = append(overwrites, memberAllowOverwrite(friendID))
	}

	return overwrites
}

func memberAllowOverwrite(userID int64) *discordgo.PermissionOverwrite {
	return &discordgo.PermissionOverwrite{
		ID:    strconv.FormatInt(userID, 10),
		Type:  discordgo.PermissionOverwriteTypeMember,
		Allow: PartyChannelPermissions,
	}
}
