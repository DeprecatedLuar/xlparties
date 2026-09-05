package commands

import (
	"fmt"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/messages"
	"xlparties/internal/party"
	"xlparties/internal/store"
)

func handlePartyLimit(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store, pm *party.Manager) {
	channelID, ok := ownedPartyChannel(s, i, st)
	if !ok {
		return
	}

	limit, ok := limitOption(i)
	if !ok {
		respondEphemeral(s, i, messages.MissingLimitOption)
		return
	}

	if err := pm.SetUserLimit(channelID, limit); err != nil {
		logger.Error("party_limit: set user limit", "error", err)
		respondPublic(s, i, fmt.Sprintf(messages.FailedSetPartyLimit, limit))
		return
	}

	if limit == 0 {
		respondPublic(s, i, messages.PartyLimitCleared)
		return
	}
	respondPublic(s, i, fmt.Sprintf(messages.PartyLimitSet, limit))
}

func limitOption(i *discordgo.InteractionCreate) (int, bool) {
	for _, opt := range i.ApplicationCommandData().Options {
		if opt.Name == "limit" {
			return int(opt.IntValue()), true
		}
	}
	return 0, false
}
