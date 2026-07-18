package commands

import (
	"fmt"
	"strconv"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/messages"
	"xlparties/internal/store"
)

// handlePartyKick disconnects the target member from the current voice channel.
// It is restricted to the party owner and only applies within the party channel.
func handlePartyKick(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store) {
	caller, target, ok := callerAndTarget(s, i)
	if !ok {
		return
	}

	channelID, err := strconv.ParseInt(i.ChannelID, 10, 64)
	if err != nil {
		logger.Error("party kick: parse channel id", "error", err)
		respondEphemeral(s, i, messages.FailedResolveChannel)
		return
	}

	activeParty, found, err := st.PartyByChannel(channelID)
	if err != nil {
		logger.Error("party kick: lookup party", "error", err)
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

	guild, err := s.State.Guild(i.GuildID)
	if err != nil {
		logger.Error("party kick: get guild", "error", err)
		respondEphemeral(s, i, messages.FailedKickUser)
		return
	}

	targetIDStr := strconv.FormatInt(target, 10)
	var inChannel bool
	for _, vs := range guild.VoiceStates {
		if vs.UserID == targetIDStr && vs.ChannelID == i.ChannelID {
			inChannel = true
			break
		}
	}
	if !inChannel {
		respondEphemeral(s, i, fmt.Sprintf(messages.UserNotPresent, target))
		return
	}

	// In discordgo, passing nil as the third argument to GuildMemberMove disconnects the member.
	if err := s.GuildMemberMove(i.GuildID, targetIDStr, nil); err != nil {
		logger.Error("party kick: move member", "error", err)
		respondEphemeral(s, i, messages.FailedKickUser)
		return
	}

	logger.Info("party kick: user kicked", "channel", channelID, "caller", caller, "target", target)
	respondPublic(s, i, fmt.Sprintf(messages.UserKicked, target))
}
