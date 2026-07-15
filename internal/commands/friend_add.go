package commands

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/store"
)

func handleFriendAdd(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store) {
	caller, target, ok := callerAndTarget(s, i)
	if !ok {
		return
	}
	if err := st.UpsertFriend(caller, target); err != nil {
		log.Printf("friend_add: %v", err)
		respondEphemeral(s, i, "failed to add friend")
		return
	}
	respondEphemeral(s, i, fmt.Sprintf("added <@%d> as a friend", target))
}
