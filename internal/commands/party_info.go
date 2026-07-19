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

	respondEphemeral(s, i, fmt.Sprintf(messages.PartyInfoHeader,
		partyModeLabel[activeParty.AccessMode],
		overrideList(allowedIDs),
		overrideList(blockedIDs),
	))
}

func overrideList(ids []int64) string {
	if len(ids) == 0 {
		return messages.NoOverrides
	}
	return mentionList(ids)
}
