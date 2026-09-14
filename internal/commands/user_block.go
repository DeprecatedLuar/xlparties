package commands

import (
	"fmt"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/messages"
	"xlparties/internal/party"
	"xlparties/internal/store"
)

func handleUserBlock(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store, pm *party.Manager) {
	caller, target, ok := callerAndTarget(s, i)
	if !ok {
		return
	}
	if err := st.UpsertBlock(caller, target); err != nil {
		logger.Error("user_block", "error", err)
		respondEphemeral(s, i, messages.FailedAddEnemy)
		return
	}
	if err := pm.RewriteAffectedChannels(caller); err != nil {
		logger.Error("user_block: rewrite affected channels", "caller", caller, "error", err)
	}

	isFriend, err := st.IsFriend(caller, target)
	if err != nil {
		logger.Error("user_block: check friend status", "error", err)
	}
	if isFriend {
		respondEphemeral(s, i, fmt.Sprintf(messages.EnemyAddedStillFriend, target))
		return
	}
	respondEphemeral(s, i, fmt.Sprintf(messages.EnemyAdded, target))
}
