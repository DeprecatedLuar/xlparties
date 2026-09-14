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
	// InviteToParty can make several sequential Discord REST calls (channel
	// lookup, overwrite grant, invite-code creation, DM), easily exceeding
	// Discord's 3-second ACK deadline - defer immediately so that runs
	// against a 15-minute budget instead. Every response below therefore
	// goes through followupThenClearPlaceholder rather than
	// respondEphemeral/respondPublic, since the outcome (not known until
	// InviteToParty returns) decides whether the reply is public or
	// ephemeral, and only a followup message - not an edited deferred
	// response - can set that per-response.
	if err := deferEphemeral(s, i); err != nil {
		return
	}

	caller, target, failure, ok := resolveCallerAndTarget(s, i)
	if !ok {
		followupThenClearPlaceholder(s, i, failure, discordgo.MessageFlagsEphemeral)
		return
	}

	channelID, err := strconv.ParseInt(i.ChannelID, 10, 64)
	if err != nil {
		logger.Error("party invite: parse channel id", "error", err)
		followupThenClearPlaceholder(s, i, messages.FailedResolveChannel, discordgo.MessageFlagsEphemeral)
		return
	}

	if _, found, err := st.PartyByChannel(channelID); err != nil {
		logger.Error("party invite: lookup party", "error", err)
		followupThenClearPlaceholder(s, i, messages.FailedLookupParty, discordgo.MessageFlagsEphemeral)
		return
	} else if !found {
		followupThenClearPlaceholder(s, i, messages.NotInParty, discordgo.MessageFlagsEphemeral)
		return
	}

	guild, err := s.State.Guild(i.GuildID)
	if err != nil {
		logger.Error("party invite: get guild", "error", err)
		followupThenClearPlaceholder(s, i, messages.FailedInviteUser, discordgo.MessageFlagsEphemeral)
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
		followupThenClearPlaceholder(s, i, messages.MustBeInPartyChannel, discordgo.MessageFlagsEphemeral)
		return
	}

	outcome, err := pm.InviteToParty(channelID, caller, target)
	if err != nil {
		logger.Error("party invite: invite to party", "error", err)
		followupThenClearPlaceholder(s, i, messages.FailedInviteUser, discordgo.MessageFlagsEphemeral)
		return
	}

	switch outcome {
	case party.InviteGranted:
		followupThenClearPlaceholder(s, i, fmt.Sprintf(messages.PartyInviteSent, target), 0)
	case party.InviteRefused:
		followupThenClearPlaceholder(s, i, fmt.Sprintf(messages.PartyInviteRefused, target), discordgo.MessageFlagsEphemeral)
	}
}
