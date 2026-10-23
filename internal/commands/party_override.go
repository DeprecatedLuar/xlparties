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
	channelID, ok := ownedPartyChannel(s, i, st)
	if !ok {
		return
	}
	call, ok := callerAndTargets(s, i)
	if !ok {
		return
	}

	actionVerb := overrideType
	successMessage := messages.UserAllowed
	if overrideType == overrideTypeDeny {
		actionVerb = "block"
		successMessage = messages.UserDenied
	}

	respondEphemeral(s, i, call.reply(func(target int64) string {
		if err := applyOverride(s, st, pm, i.ChannelID, channelID, target, overrideType); err != nil {
			logger.Error("party override", "override_type", overrideType, "target", target, "error", err)
			return fmt.Sprintf(messages.FailedOverrideUser, actionVerb)
		}
		logger.Info("party override set", "channel", channelID, "target", target, "override_type", overrideType)
		return fmt.Sprintf(successMessage, target)
	}))
}

// applyOverride sets target's member overwrite on the party channel and
// persists the manual decision. A manual allow or deny is a standing
// decision on this exact target; any pending /party_invite is superseded
// either way (allow: they no longer need the temp grant; deny: it must not
// survive to expire and wipe the deny overwrite it left behind).
func applyOverride(s *discordgo.Session, st *store.Store, pm *party.Manager, channelIDStr string, channelID, target int64, overrideType string) error {
	var allow, deny int64
	if overrideType == overrideTypeAllow {
		allow = party.PartyChannelPermissions
	} else {
		deny = party.PartyChannelPermissions
	}

	if err := s.ChannelPermissionSet(channelIDStr, strconv.FormatInt(target, 10), discordgo.PermissionOverwriteTypeMember, allow, deny); err != nil {
		return fmt.Errorf("set channel permission: %w", err)
	}
	if err := st.UpsertOverride(channelID, target, overrideType); err != nil {
		return fmt.Errorf("upsert override: %w", err)
	}
	if err := pm.ClearPendingInvite(channelID, target); err != nil {
		logger.Error("party override: clear pending invite", "channel", channelID, "target", target, "error", err)
	}
	return nil
}
