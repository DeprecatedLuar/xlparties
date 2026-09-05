package commands

import (
	"fmt"
	"strconv"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/messages"
	"xlparties/internal/store"
)

func handlePartyInfo(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store) {
	caller, err := callerID(i)
	if err != nil {
		logger.Error("party_info: resolve caller id", "error", err)
		respondEphemeral(s, i, messages.FailedResolveCaller)
		return
	}

	channelID, err := strconv.ParseInt(i.ChannelID, 10, 64)
	if err != nil {
		logger.Error("party_info: parse channel id", "error", err)
		respondEphemeral(s, i, messages.FailedResolveChannel)
		return
	}

	activeParty, found, err := st.PartyByChannel(channelID)
	if err != nil {
		logger.Error("party_info: lookup party", "error", err)
		respondEphemeral(s, i, messages.FailedLookupParty)
		return
	}
	if !found {
		respondEphemeral(s, i, messages.NotInParty)
		return
	}

	overrides, err := st.OverridesForChannel(channelID)
	if err != nil {
		logger.Error("party_info: load overrides", "error", err)
		respondEphemeral(s, i, messages.FailedLookupParty)
		return
	}

	var allowedIDs, blockedIDs []int64
	for _, o := range overrides {
		if o.Type == overrideTypeAllow {
			allowedIDs = append(allowedIDs, o.UserID)
		} else {
			blockedIDs = append(blockedIDs, o.UserID)
		}
	}

	preset, found, err := st.PresetForUser(caller)
	if err != nil {
		logger.Error("party_info: lookup preset", "error", err)
		respondEphemeral(s, i, messages.FailedLookupParty)
		return
	}
	presetLine := fmt.Sprintf(messages.NoPartyPreset, partyModeLabel[store.DefaultAccessMode])
	if found {
		presetLine = fmt.Sprintf(messages.PartyPresetCurrent, partyModeLabel[preset])
	}

	presetLimit, found, err := st.PresetLimitForUser(caller)
	if err != nil {
		logger.Error("party_info: lookup preset limit", "error", err)
		respondEphemeral(s, i, messages.FailedLookupParty)
		return
	}
	presetLimitLine := messages.NoPartyPresetLimit
	if found && presetLimit != 0 {
		presetLimitLine = fmt.Sprintf(messages.PartyPresetLimitCurrent, presetLimit)
	}

	channel, err := s.Channel(i.ChannelID)
	if err != nil {
		logger.Error("party_info: fetch channel", "error", err)
		respondEphemeral(s, i, messages.FailedLookupParty)
		return
	}
	limitDisplay := messages.PartyInfoNoLimit
	if channel.UserLimit != 0 {
		limitDisplay = strconv.Itoa(channel.UserLimit)
	}

	respondEphemeral(s, i, fmt.Sprintf(messages.PartyInfoHeader,
		partyModeLabel[activeParty.AccessMode],
		limitDisplay,
		overrideList(allowedIDs),
		overrideList(blockedIDs),
		presetLine,
		presetLimitLine,
	))
}

func overrideList(ids []int64) string {
	if len(ids) == 0 {
		return messages.NoOverrides
	}
	return mentionList(ids)
}
