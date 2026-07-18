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

	ids, err := st.FriendIDs(caller)
	if err != nil {
		logger.Error("friend_list", "error", err)
		respondEphemeral(s, i, messages.FailedListFriends)
		return
	}
	if len(ids) == 0 {
		respondEphemeral(s, i, messages.NoFriends)
		return
	}
	respondEphemeral(s, i, fmt.Sprintf(messages.FriendListHeader, mentionList(ids)))
}

func mentionList(ids []int64) string {
	mentions := make([]string, len(ids))
	for idx, id := range ids {
		mentions[idx] = fmt.Sprintf("<@%d>", id)
	}
	return strings.Join(mentions, "\n")
}
