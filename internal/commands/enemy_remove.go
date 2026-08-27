package commands

import (
	"fmt"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/messages"
	"xlparties/internal/party"
	"xlparties/internal/store"
)

func handleEnemyRemove(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store, pm *party.Manager) {
	caller, target, ok := callerAndTarget(s, i)
	if !ok {
		return
	}
	if err := st.RemoveBlock(caller, target); err != nil {
		logger.Error("enemy_remove", "error", err)
		respondEphemeral(s, i, messages.FailedRemoveEnemy)
		return
	}
	if err := pm.RewriteAffectedChannels(caller); err != nil {
		logger.Error("enemy_remove: rewrite affected channels", "caller", caller, "error", err)
	}
	respondEphemeral(s, i, fmt.Sprintf(messages.EnemyRemoved, target))
}
