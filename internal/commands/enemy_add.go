package commands

import (
	"fmt"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/messages"
	"xlparties/internal/store"
)

func handleEnemyAdd(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store) {
	caller, target, ok := callerAndTarget(s, i)
	if !ok {
		return
	}
	if err := st.UpsertBlock(caller, target); err != nil {
		logger.Error("enemy_add", "error", err)
		respondEphemeral(s, i, messages.FailedAddEnemy)
		return
	}
	respondEphemeral(s, i, fmt.Sprintf(messages.EnemyAdded, target))
}
