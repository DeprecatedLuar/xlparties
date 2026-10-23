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

// handlePartyKick disconnects the target members from the current voice channel.
// It is restricted to the party owner and only applies within the party channel.
func handlePartyKick(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store, _ *party.Manager) {
	channelID, ok := ownedPartyChannel(s, i, st)
	if !ok {
		return
	}
	call, ok := callerAndTargets(s, i)
	if !ok {
		return
	}

	respondEphemeral(s, i, call.reply(func(target int64) string {
		targetIDStr := strconv.FormatInt(target, 10)
		inChannel, err := userInVoiceChannel(s, i.GuildID, i.ChannelID, targetIDStr)
		if err != nil {
			logger.Error("party kick: get guild", "error", err)
			return messages.FailedKickUser
		}
		if !inChannel {
			return fmt.Sprintf(messages.UserNotPresent, target)
		}

		// In discordgo, passing nil as the third argument to GuildMemberMove disconnects the member.
		if err := s.GuildMemberMove(i.GuildID, targetIDStr, nil); err != nil {
			logger.Error("party kick: move member", "error", err)
			return messages.FailedKickUser
		}

		logger.Info("party kick: user kicked", "channel", channelID, "caller", call.caller, "target", target)
		return fmt.Sprintf(messages.UserKicked, target)
	}))
}
