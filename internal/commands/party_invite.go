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

// handlePartyInvite grants targetID temporary access to the caller's current
// party, tied to their presence: the access-decision itself (already
// allowed, refused, or granted) is made by party.Manager.InviteToParty.
// Unlike /party_allow and /party_ban, any member currently connected to the
// party voice channel may invite - not just the owner.
func handlePartyInvite(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store, pm *party.Manager) {
	caller, target, ok := callerAndTarget(s, i)
	if !ok {
		return
	}

	channelID, err := strconv.ParseInt(i.ChannelID, 10, 64)
	if err != nil {
		logger.Error("party invite: parse channel id", "error", err)
		respondEphemeral(s, i, messages.FailedResolveChannel)
		return
	}

	if _, found, err := st.PartyByChannel(channelID); err != nil {
		logger.Error("party invite: lookup party", "error", err)
		respondEphemeral(s, i, messages.FailedLookupParty)
		return
	} else if !found {
		respondEphemeral(s, i, messages.NotInParty)
		return
	}

	guild, err := s.State.Guild(i.GuildID)
	if err != nil {
		logger.Error("party invite: get guild", "error", err)
		respondEphemeral(s, i, messages.FailedInviteUser)
		return
	}
	callerIDStr := strconv.FormatInt(caller, 10)
	var callerConnected bool
	for _, vs := range guild.VoiceStates {
		if vs.UserID == callerIDStr && vs.ChannelID == i.ChannelID {
			callerConnected = true
			break
		}
	}
	if !callerConnected {
		respondEphemeral(s, i, messages.MustBeInPartyChannel)
		return
	}

	outcome, err := pm.InviteToParty(channelID, caller, target)
	if err != nil {
		logger.Error("party invite: invite to party", "error", err)
		respondEphemeral(s, i, messages.FailedInviteUser)
		return
	}

	switch outcome {
	case party.InviteGranted:
		respondPublic(s, i, fmt.Sprintf(messages.PartyInviteSent, target))
	case party.InviteAlreadyHasAccess:
		respondEphemeral(s, i, fmt.Sprintf(messages.PartyInviteAlreadyHasAccess, target))
	case party.InviteRefused:
		respondEphemeral(s, i, fmt.Sprintf(messages.PartyInviteRefused, target))
	}
}
