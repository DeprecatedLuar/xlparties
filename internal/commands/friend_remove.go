package commands

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/store"
)

func handleFriendRemove(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store) {
	caller, target, ok := callerAndTarget(s, i)
	if !ok {
		return
	}
	if err := st.RemoveFriend(caller, target); err != nil {
		log.Printf("friend_remove: %v", err)
		respondEphemeral(s, i, "failed to remove friend")
		return
	}
	respondEphemeral(s, i, fmt.Sprintf("removed <@%d> as a friend", target))
}
