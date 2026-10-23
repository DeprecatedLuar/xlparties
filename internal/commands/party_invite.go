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

// handlePartyInvite grants each target temporary access to the caller's
// current party, tied to their presence: the access-decision itself
// (already allowed, refused, or granted) is made by
// party.Manager.InviteToParty. Unlike /party_allow and /party_ban, any
// member currently connected to the party voice channel may invite - not
// just the owner.
func handlePartyInvite(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store, pm *party.Manager) {
	// InviteToParty can make several sequential Discord REST calls (channel
	// lookup, overwrite grant, invite-code creation, DM) per target, easily
	// exceeding Discord's 3-second ACK deadline - defer immediately so that
	// runs against a 15-minute budget instead. Every response is ephemeral,
	// so each goes through editDeferred.
	if err := deferEphemeral(s, i); err != nil {
		return
	}

	call, failure, ok := resolveTargetedCall(s, i)
	if !ok {
		editDeferred(s, i, failure)
		return
	}

	channelID, err := strconv.ParseInt(i.ChannelID, 10, 64)
	if err != nil {
		logger.Error("party invite: parse channel id", "error", err)
		editDeferred(s, i, messages.FailedResolveChannel)
		return
	}

	if _, found, err := st.PartyByChannel(channelID); err != nil {
		logger.Error("party invite: lookup party", "error", err)
		editDeferred(s, i, messages.FailedLookupParty)
		return
	} else if !found {
		editDeferred(s, i, messages.NotInParty)
		return
	}

	callerConnected, err := userInVoiceChannel(s, i.GuildID, i.ChannelID, strconv.FormatInt(call.caller, 10))
	if err != nil {
		logger.Error("party invite: get guild", "error", err)
		editDeferred(s, i, messages.FailedInviteUser)
		return
	}
	if !callerConnected {
		editDeferred(s, i, messages.MustBeInPartyChannel)
		return
	}

	editDeferred(s, i, call.reply(func(target int64) string {
		outcome, err := pm.InviteToParty(channelID, call.caller, target)
		if err != nil {
			logger.Error("party invite: invite to party", "target", target, "error", err)
			return messages.FailedInviteUser
		}
		if outcome == party.InviteGranted {
			return fmt.Sprintf(messages.PartyInviteSent, target)
		}
		return fmt.Sprintf(messages.PartyInviteRefused, target)
	}))
}
