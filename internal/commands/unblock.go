package commands

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/messages"
	"xlparties/internal/store"
)

func handleUnblock(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store) {
	caller, target, ok := callerAndTarget(s, i)
	if !ok {
		return
	}
	if err := st.RemoveBlock(caller, target); err != nil {
		log.Printf("unblock: %v", err)
		respondEphemeral(s, i, messages.FailedUnblockUser)
		return
	}
	respondEphemeral(s, i, fmt.Sprintf(messages.UserUnblocked, target))
}
