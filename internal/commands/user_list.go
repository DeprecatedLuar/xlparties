package commands

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/messages"
	"xlparties/internal/store"
)

func handleUserList(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store) {
	caller, err := callerID(i)
	if err != nil {
		logger.Error("user_list: resolve caller id", "error", err)
		respondEphemeral(s, i, messages.FailedResolveCaller)
		return
	}

	bestieIDs, err := st.AllowedFavoriteIDs(caller)
	if err != nil {
		logger.Error("user_list: besties", "error", err)
		respondEphemeral(s, i, messages.FailedListRelationships)
		return
	}
	friendOnlyIDs, err := st.FriendOnlyIDs(caller)
	if err != nil {
		logger.Error("user_list: friends", "error", err)
		respondEphemeral(s, i, messages.FailedListRelationships)
		return
	}
	enemyOnlyIDs, err := st.EnemyIDs(caller)
	if err != nil {
		logger.Error("user_list: enemies", "error", err)
		respondEphemeral(s, i, messages.FailedListRelationships)
		return
	}
	frenemyIDs, err := st.FrenemyIDs(caller)
	if err != nil {
		logger.Error("user_list: frenemies", "error", err)
		respondEphemeral(s, i, messages.FailedListRelationships)
		return
	}
	bestFrenemyIDs, err := st.BestFrenemyIDs(caller)
	if err != nil {
		logger.Error("user_list: best frenemies", "error", err)
		respondEphemeral(s, i, messages.FailedListRelationships)
		return
	}

	if len(bestieIDs) == 0 && len(friendOnlyIDs) == 0 && len(enemyOnlyIDs) == 0 && len(frenemyIDs) == 0 && len(bestFrenemyIDs) == 0 {
		respondEphemeral(s, i, messages.NoRelationships)
		return
	}

	var sections []string
	sections = append(sections, fmt.Sprintf(messages.BestieListHeader, mentionListOr(bestieIDs, messages.NoOverrides)))
	sections = append(sections, fmt.Sprintf(messages.FriendListHeader, mentionListOr(friendOnlyIDs, messages.NoOverrides)))
	sections = append(sections, fmt.Sprintf(messages.EnemyListHeader, mentionListOr(enemyOnlyIDs, messages.NoOverrides)))
	if len(frenemyIDs) > 0 {
		sections = append(sections, fmt.Sprintf(messages.FrenemyListHeader, mentionList(frenemyIDs)))
	}
	if len(bestFrenemyIDs) > 0 {
		sections = append(sections, fmt.Sprintf(messages.BestFrenemyListHeader, mentionList(bestFrenemyIDs)))
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
