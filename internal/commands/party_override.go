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

const (
	overrideTypeAllow = "allow"
	overrideTypeDeny  = "deny"
)

func handlePartyAllow(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store, pm *party.Manager) {
	handlePartyOverride(s, i, st, pm, overrideTypeAllow)
}

func handlePartyBlock(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store, pm *party.Manager) {
	handlePartyOverride(s, i, st, pm, overrideTypeDeny)
}

func handlePartyOverride(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store, pm *party.Manager, overrideType string) {
	caller, target, ok := callerAndTarget(s, i)
	if !ok {
		return
	}

	channelID, err := strconv.ParseInt(i.ChannelID, 10, 64)
	if err != nil {
		logger.Error("party override: parse channel id", "override_type", overrideType, "error", err)
		respondEphemeral(s, i, messages.FailedResolveChannel)
		return
	}

	activeParty, found, err := st.PartyByChannel(channelID)
	if err != nil {
		logger.Error("party override: lookup party", "override_type", overrideType, "error", err)
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

	var allow, deny int64
	if overrideType == overrideTypeAllow {
		allow = party.PartyChannelPermissions
	} else {
		deny = party.PartyChannelPermissions
	}
	actionVerb := overrideType
	if actionVerb == overrideTypeDeny {
		actionVerb = "block"
	}

	targetID := strconv.FormatInt(target, 10)
	if err := s.ChannelPermissionSet(i.ChannelID, targetID, discordgo.PermissionOverwriteTypeMember, allow, deny); err != nil {
		logger.Error("party override: set channel permission", "override_type", overrideType, "error", err)
		respondEphemeral(s, i, fmt.Sprintf(messages.FailedOverrideUser, actionVerb))
		return
	}
	if err := st.UpsertOverride(channelID, target, overrideType); err != nil {
		logger.Error("party override: upsert override", "override_type", overrideType, "error", err)
		respondEphemeral(s, i, fmt.Sprintf(messages.FailedOverrideUser, actionVerb))
		return
	}

	// A manual allow or deny is a standing decision on this exact target;
	// any pending /party_invite is superseded either way (allow: they no
	// longer need the temp grant; deny: it must not survive to expire and
	// wipe the deny overwrite it left behind).
	if err := pm.ClearPendingInvite(channelID, target); err != nil {
		logger.Error("party override: clear pending invite", "channel", channelID, "target", target, "error", err)
	}

	logger.Info("party override set", "channel", channelID, "target", target, "override_type", overrideType)

	if overrideType == overrideTypeAllow {
		respondPublic(s, i, fmt.Sprintf(messages.UserAllowed, target))
	} else {
		respondEphemeral(s, i, fmt.Sprintf(messages.UserDenied, target))
	}
}
