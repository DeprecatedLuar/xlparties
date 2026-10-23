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

// handlePartyBan denies users future access to the party channel and also
// disconnects/kicks them immediately if they are currently connected to it.
func handlePartyBan(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store, pm *party.Manager) {
	channelID, ok := ownedPartyChannel(s, i, st)
	if !ok {
		return
	}
	call, ok := callerAndTargets(s, i)
	if !ok {
		return
	}

	respondEphemeral(s, i, call.reply(func(target int64) string {
		if err := applyOverride(s, st, pm, i.ChannelID, channelID, target, overrideTypeDeny); err != nil {
			logger.Error("party ban", "target", target, "error", err)
			return messages.FailedBanUser
		}

		// The overwrite is already set, so a failed eject is logged, not surfaced.
		targetIDStr := strconv.FormatInt(target, 10)
		inChannel, err := userInVoiceChannel(s, i.GuildID, i.ChannelID, targetIDStr)
		if err != nil {
			logger.Error("party ban: get guild state for kick check", "error", err)
		}
		if inChannel {
			if err := s.GuildMemberMove(i.GuildID, targetIDStr, nil); err != nil {
				logger.Error("party ban: move member (kick)", "error", err)
			}
		}

		logger.Info("party ban: user banned and kicked", "channel", channelID, "caller", call.caller, "target", target)
		return fmt.Sprintf(messages.UserBanned, target)
	}))
}
