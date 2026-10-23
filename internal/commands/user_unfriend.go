package commands

import (
	"fmt"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/messages"
	"xlparties/internal/party"
	"xlparties/internal/store"
)

func handleUserUnfriend(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store, pm *party.Manager) {
	runUserEdit(s, i, pm, "user_unfriend", func(caller, target int64) string {
		if err := st.RemoveFriend(caller, target); err != nil {
			logger.Error("user_unfriend", "error", err)
			return messages.FailedRemoveFriend
		}
		return fmt.Sprintf(messages.FriendRemoved, target)
	})
}
