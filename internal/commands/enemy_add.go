package commands

import (
	"fmt"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/messages"
	"xlparties/internal/party"
	"xlparties/internal/store"
)

func handleEnemyAdd(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store, pm *party.Manager) {
	caller, target, ok := callerAndTarget(s, i)
	if !ok {
		return
	}
	if err := st.UpsertBlock(caller, target); err != nil {
		logger.Error("enemy_add", "error", err)
		respondEphemeral(s, i, messages.FailedAddEnemy)
		return
	}
	if err := pm.RewriteAffectedChannels(caller); err != nil {
		logger.Error("enemy_add: rewrite affected channels", "caller", caller, "error", err)
	}

	isFriend, err := st.IsFriend(caller, target)
	if err != nil {
		logger.Error("enemy_add: check friend status", "error", err)
	}
	if isFriend {
		respondEphemeral(s, i, fmt.Sprintf(messages.EnemyAddedStillFriend, target))
		return
	}
	respondEphemeral(s, i, fmt.Sprintf(messages.EnemyAdded, target))
}
