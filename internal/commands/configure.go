package commands

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/messages"
	"xlparties/internal/store"
)

const (
	subcommandWatchChannel = "watch_channel"
	subcommandCategory     = "category"
)

func handleConfigure(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store) {
	options := i.ApplicationCommandData().Options
	if len(options) != 1 {
		respondEphemeral(s, i, messages.ExpectedOneSubcommand)
		return
	}
	sub := options[0]

	switch sub.Name {
	case subcommandWatchChannel:
		handleConfigureWatchChannel(s, i, st, sub)
	case subcommandCategory:
		handleConfigureCategory(s, i, st, sub)
	default:
		respondEphemeral(s, i, messages.UnknownSubcommand)
	}
}

func handleConfigureWatchChannel(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store, sub *discordgo.ApplicationCommandInteractionDataOption) {
	channel := sub.Options[0].ChannelValue(s)
	if err := st.SetConfig(store.ConfigKeyWatchChannel, channel.ID); err != nil {
		log.Printf("configure watch_channel: %v", err)
		respondEphemeral(s, i, messages.FailedSaveWatchChan)
		return
	}
	respondEphemeral(s, i, fmt.Sprintf(messages.WatchChannelSet, channel.ID))
}

func handleConfigureCategory(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store, sub *discordgo.ApplicationCommandInteractionDataOption) {
	channel := sub.Options[0].ChannelValue(s)
	if err := st.SetConfig(store.ConfigKeyCategory, channel.ID); err != nil {
		log.Printf("configure category: %v", err)
		respondEphemeral(s, i, messages.FailedSaveCategory)
		return
	}
	respondEphemeral(s, i, fmt.Sprintf(messages.CategorySet, channel.ID))
}
