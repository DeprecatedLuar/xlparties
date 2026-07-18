package commands

import (
	"fmt"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/messages"
	"xlparties/internal/store"
)

func handleEnemyRemove(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store) {
	caller, target, ok := callerAndTarget(s, i)
	if !ok {
		return
	}
	if err := st.RemoveBlock(caller, target); err != nil {
		logger.Error("enemy_remove", "error", err)
		respondEphemeral(s, i, messages.FailedRemoveEnemy)
		return
	}
	respondEphemeral(s, i, fmt.Sprintf(messages.EnemyRemoved, target))
}
