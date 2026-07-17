package commands

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/messages"
	"xlparties/internal/store"
)

func handleBlock(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store) {
	caller, target, ok := callerAndTarget(s, i)
	if !ok {
		return
	}
	if err := st.UpsertBlock(caller, target); err != nil {
		log.Printf("block: %v", err)
		respondEphemeral(s, i, messages.FailedBlockUser)
		return
	}
	respondEphemeral(s, i, fmt.Sprintf(messages.UserBlocked, target))
}
