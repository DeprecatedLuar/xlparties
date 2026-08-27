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

	blockedIDs, err := st.BlockIDs(caller)
	if err != nil {
		logger.Error("enemy_list", "error", err)
		respondEphemeral(s, i, messages.FailedListEnemies)
		return
	}
	frenemyIDs, err := st.FrenemyIDs(caller)
	if err != nil {
		logger.Error("enemy_list: frenemies", "error", err)
		respondEphemeral(s, i, messages.FailedListEnemies)
		return
	}
	if len(blockedIDs) == 0 {
		respondEphemeral(s, i, messages.NoEnemies)
		return
	}

	isFrenemy := make(map[int64]bool, len(frenemyIDs))
	for _, id := range frenemyIDs {
		isFrenemy[id] = true
	}
	var enemyOnlyIDs []int64
	for _, id := range blockedIDs {
		if !isFrenemy[id] {
			enemyOnlyIDs = append(enemyOnlyIDs, id)
		}
	}

	body := fmt.Sprintf(messages.EnemyListHeader, mentionListOr(enemyOnlyIDs, messages.NoOverrides))
	if len(frenemyIDs) > 0 {
		body += fmt.Sprintf(messages.FrenemyListSection, mentionList(frenemyIDs))
	}
	respondEphemeral(s, i, body)
}
