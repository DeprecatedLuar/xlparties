package commands

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/messages"
	"xlparties/internal/store"
)

func handleFriendRemove(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store) {
	caller, target, ok := callerAndTarget(s, i)
	if !ok {
		return
	}
	if err := st.RemoveFriend(caller, target); err != nil {
		log.Printf("friend_remove: %v", err)
		respondEphemeral(s, i, messages.FailedRemoveFriend)
		return
	}
	respondEphemeral(s, i, fmt.Sprintf(messages.FriendRemoved, target))
}
