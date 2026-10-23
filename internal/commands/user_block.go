package commands

import (
	"fmt"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/messages"
	"xlparties/internal/party"
	"xlparties/internal/store"
)

func handleUserBlock(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store, pm *party.Manager) {
	runUserEdit(s, i, pm, "user_block", func(caller, target int64) string {
		if err := st.UpsertBlock(caller, target); err != nil {
			logger.Error("user_block", "error", err)
			return messages.FailedAddEnemy
		}
		isFriend, err := st.IsFriend(caller, target)
		if err != nil {
			logger.Error("user_block: check friend status", "error", err)
		}
		if isFriend {
			return fmt.Sprintf(messages.EnemyAddedStillFriend, target)
		}
		return fmt.Sprintf(messages.EnemyAdded, target)
	})
}
