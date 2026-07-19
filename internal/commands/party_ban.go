package commands

import (
	"fmt"
	"strconv"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/messages"
	"xlparties/internal/party"
	"xlparties/internal/store"
)

// handlePartyBan denies a user future access to the party channel and also
// disconnects/kicks them immediately if they are currently connected to it.
func handlePartyBan(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store, pm *party.Manager) {
	caller, target, ok := callerAndTarget(s, i)
	if !ok {
		return
	}

	channelID, err := strconv.ParseInt(i.ChannelID, 10, 64)
	if err != nil {
		logger.Error("party ban: parse channel id", "error", err)
		respondEphemeral(s, i, messages.FailedResolveChannel)
		return
	}

	activeParty, found, err := st.PartyByChannel(channelID)
	if err != nil {
		logger.Error("party ban: lookup party", "error", err)
		respondEphemeral(s, i, messages.FailedLookupParty)
		return
	}
	if !found {
		respondEphemeral(s, i, messages.NotInParty)
		return
	}
	if activeParty.OwnerID != caller {
		respondEphemeral(s, i, fmt.Sprintf(messages.MustBeOwner, activeParty.OwnerID))
		return
	}

	// 1. Deny access (permission overwrites)
	targetIDStr := strconv.FormatInt(target, 10)
	denyPerms := int64(party.PartyChannelPermissions)
	if err := s.ChannelPermissionSet(i.ChannelID, targetIDStr, discordgo.PermissionOverwriteTypeMember, 0, denyPerms); err != nil {
		logger.Error("party ban: set channel permission", "error", err)
		respondEphemeral(s, i, messages.FailedBanUser)
		return
	}

	// 2. Persist the override
	if err := st.UpsertOverride(channelID, target, overrideTypeDeny); err != nil {
		logger.Error("party ban: upsert override", "error", err)
		respondEphemeral(s, i, messages.FailedBanUser)
		return
	}

	if err := pm.ClearPendingInvite(channelID, target); err != nil {
		logger.Error("party ban: clear pending invite", "channel", channelID, "target", target, "error", err)
	}

	// 3. Eject them if they are currently connected
	guild, err := s.State.Guild(i.GuildID)
	if err == nil {
		var inChannel bool
		for _, vs := range guild.VoiceStates {
			if vs.UserID == targetIDStr && vs.ChannelID == i.ChannelID {
				inChannel = true
				break
			}
		}
		if inChannel {
			if err := s.GuildMemberMove(i.GuildID, targetIDStr, nil); err != nil {
				logger.Error("party ban: move member (kick)", "error", err)
				// Note: We don't fail the command if the kick fails, since the ban (overwrite)
				// was already successfully set. We just log the warning.
			}
		}
	} else {
		logger.Error("party ban: get guild state for kick check", "error", err)
	}

	logger.Info("party ban: user banned and kicked", "channel", channelID, "caller", caller, "target", target)
	respondPublic(s, i, fmt.Sprintf(messages.UserBanned, target))
}
