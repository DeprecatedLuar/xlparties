package commands

import (
	"fmt"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/messages"
	"xlparties/internal/party"
	"xlparties/internal/store"
)

func handleUserUnblock(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store, pm *party.Manager) {
	runUserEdit(s, i, pm, "user_unblock", func(caller, target int64) string {
		if err := st.RemoveBlock(caller, target); err != nil {
			logger.Error("user_unblock", "error", err)
			return messages.FailedRemoveEnemy
		}
		return fmt.Sprintf(messages.EnemyRemoved, target)
	})
}
