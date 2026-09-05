package commands

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/messages"
	"xlparties/internal/store"
)

func handleRelationships(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store) {
	caller, err := callerID(i)
	if err != nil {
		logger.Error("relationships: resolve caller id", "error", err)
		respondEphemeral(s, i, messages.FailedResolveCaller)
		return
	}

	friendOnlyIDs, err := st.AllowedFriendIDs(caller)
	if err != nil {
		logger.Error("relationships: friends", "error", err)
		respondEphemeral(s, i, messages.FailedListRelationships)
		return
	}
	blockedIDs, err := st.BlockIDs(caller)
	if err != nil {
		logger.Error("relationships: enemies", "error", err)
		respondEphemeral(s, i, messages.FailedListRelationships)
		return
	}
	frenemyIDs, err := st.FrenemyIDs(caller)
	if err != nil {
		logger.Error("relationships: frenemies", "error", err)
		respondEphemeral(s, i, messages.FailedListRelationships)
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

	if len(friendOnlyIDs) == 0 && len(enemyOnlyIDs) == 0 && len(frenemyIDs) == 0 {
		respondEphemeral(s, i, messages.NoRelationships)
		return
	}

	var sections []string
	sections = append(sections, fmt.Sprintf(messages.FriendListHeader, mentionListOr(friendOnlyIDs, messages.NoOverrides)))
	sections = append(sections, fmt.Sprintf(messages.EnemyListHeader, mentionListOr(enemyOnlyIDs, messages.NoOverrides)))
	if len(frenemyIDs) > 0 {
		sections = append(sections, fmt.Sprintf(messages.FrenemyListHeader, mentionList(frenemyIDs)))
	}
	respondEphemeral(s, i, strings.Join(sections, "\n\n"))
}

func mentionListOr(ids []int64, empty string) string {
	if len(ids) == 0 {
		return empty
	}
	return mentionList(ids)
}

func mentionList(ids []int64) string {
	mentions := make([]string, len(ids))
	for idx, id := range ids {
		mentions[idx] = fmt.Sprintf("<@%d>", id)
	}
	return strings.Join(mentions, "\n")
}
