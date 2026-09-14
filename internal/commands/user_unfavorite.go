package commands

import (
	"fmt"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/messages"
	"xlparties/internal/party"
	"xlparties/internal/store"
)

func handleUserUnfavorite(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store, pm *party.Manager) {
	caller, target, ok := callerAndTarget(s, i)
	if !ok {
		return
	}
	if err := st.RemoveFavorite(caller, target); err != nil {
		logger.Error("user_unfavorite", "error", err)
		respondEphemeral(s, i, messages.FailedRemoveFavorite)
		return
	}
	if err := pm.RewriteAffectedChannels(caller); err != nil {
		logger.Error("user_unfavorite: rewrite affected channels", "caller", caller, "error", err)
	}
	respondEphemeral(s, i, fmt.Sprintf(messages.FavoriteRemoved, target))
}
