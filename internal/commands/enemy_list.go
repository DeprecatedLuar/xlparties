package commands

import (
	"fmt"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/messages"
	"xlparties/internal/store"
)

func handleEnemyList(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store) {
	caller, err := callerID(i)
	if err != nil {
		logger.Error("enemy_list: resolve caller id", "error", err)
		respondEphemeral(s, i, messages.FailedResolveCaller)
		return
	}

	ids, err := st.BlockIDs(caller)
	if err != nil {
		logger.Error("enemy_list", "error", err)
		respondEphemeral(s, i, messages.FailedListEnemies)
		return
	}
	if len(ids) == 0 {
		respondEphemeral(s, i, messages.NoEnemies)
		return
	}
	respondEphemeral(s, i, fmt.Sprintf(messages.EnemyListHeader, mentionList(ids)))
}
