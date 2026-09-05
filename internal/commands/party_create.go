package commands

import (
	"fmt"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/messages"
	"xlparties/internal/party"
	"xlparties/internal/store"
)

// handlePartyCreate opens (or reports) the caller's party from a slash
// command instead of the watch channel, letting mode and limit be set
// inline instead of only through a saved /party_preset. Unlike every other
// party_* command it deliberately does not use ownedPartyChannel: the
// caller is not expected to be in a party channel yet, or even in voice at
// all - CreateParty itself decides whether to move them in.
func handlePartyCreate(s *discordgo.Session, i *discordgo.InteractionCreate, _ *store.Store, pm *party.Manager) {
	caller, err := callerID(i)
	if err != nil {
		logger.Error("party_create: resolve caller id", "error", err)
		respondEphemeral(s, i, messages.FailedResolveCaller)
		return
	}

	mode, limit, limitGiven := createOptions(i)
	var limitOverride *int
	if limitGiven {
		limitOverride = &limit
	}

	channelID, alreadyExisted, err := pm.CreateParty(caller, mode, limitOverride)
	if err != nil {
		logger.Error("party_create: create party", "caller", caller, "error", err)
		respondEphemeral(s, i, messages.FailedCreateParty)
		return
	}

	if alreadyExisted {
		respondEphemeral(s, i, fmt.Sprintf(messages.PartyCreateAlreadyExists, channelID))
		return
	}
	respondEphemeral(s, i, fmt.Sprintf(messages.PartyCreateReady, channelID))
}

// createOptions reads /party_create's optional "mode" and "limit" options.
func createOptions(i *discordgo.InteractionCreate) (mode string, limit int, limitGiven bool) {
	for _, opt := range i.ApplicationCommandData().Options {
		switch opt.Name {
		case "mode":
			mode = opt.StringValue()
		case "limit":
			limit, limitGiven = int(opt.IntValue()), true
		}
	}
	return mode, limit, limitGiven
}
