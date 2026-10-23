package commands

import (
	"fmt"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/messages"
	"xlparties/internal/party"
	"xlparties/internal/store"
)

func handleUserUnfavorite(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store, pm *party.Manager) {
	runUserEdit(s, i, pm, "user_unfavorite", func(caller, target int64) string {
		if err := st.RemoveFavorite(caller, target); err != nil {
			logger.Error("user_unfavorite", "error", err)
			return messages.FailedRemoveFavorite
		}
		return fmt.Sprintf(messages.FavoriteRemoved, target)
	})
}
