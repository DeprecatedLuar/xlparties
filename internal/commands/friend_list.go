package commands

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/messages"
	"xlparties/internal/store"
)

func handleFriendList(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store) {
	caller, err := callerID(i)
	if err != nil {
		logger.Error("friend_list: resolve caller id", "error", err)
		respondEphemeral(s, i, messages.FailedResolveCaller)
		return
	}

	ids, err := st.AllowedFriendIDs(caller)
	if err != nil {
		logger.Error("friend_list", "error", err)
		respondEphemeral(s, i, messages.FailedListFriends)
		return
	}
	frenemyIDs, err := st.FrenemyIDs(caller)
	if err != nil {
		logger.Error("friend_list: frenemies", "error", err)
		respondEphemeral(s, i, messages.FailedListFriends)
		return
	}
	if len(ids) == 0 && len(frenemyIDs) == 0 {
		respondEphemeral(s, i, messages.NoFriends)
		return
	}

	body := fmt.Sprintf(messages.FriendListHeader, mentionListOr(ids, messages.NoOverrides))
	if len(frenemyIDs) > 0 {
		body += fmt.Sprintf(messages.FrenemyListSection, mentionList(frenemyIDs))
	}
	respondEphemeral(s, i, body)
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
