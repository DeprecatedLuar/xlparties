package commands

import (
	"fmt"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/messages"
	"xlparties/internal/party"
	"xlparties/internal/store"
)

func handleUserFavorite(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store, pm *party.Manager) {
	caller, target, ok := callerAndTarget(s, i)
	if !ok {
		return
	}

	alreadyFavorite, err := st.IsFavorite(caller, target)
	if err != nil {
		logger.Error("user_favorite", "error", err)
		respondEphemeral(s, i, messages.FailedAddFavorite)
		return
	}
	if alreadyFavorite {
		respondEphemeral(s, i, fmt.Sprintf(messages.AlreadyFavorite, target))
		return
	}

	isFriend, err := st.IsFriend(caller, target)
	if err != nil {
		logger.Error("user_favorite: check friend status", "error", err)
		respondEphemeral(s, i, messages.FailedAddFavorite)
		return
	}
	if !isFriend {
		if err := addFriend(s, i.GuildID, st, caller, target); err != nil {
			logger.Error("user_favorite: add friend", "error", err)
			respondEphemeral(s, i, messages.FailedAddFavorite)
			return
		}
	}

	if err := st.UpsertFavorite(caller, target); err != nil {
		logger.Error("user_favorite", "error", err)
		respondEphemeral(s, i, messages.FailedAddFavorite)
		return
	}
	if err := pm.RewriteAffectedChannels(caller); err != nil {
		logger.Error("user_favorite: rewrite affected channels", "caller", caller, "error", err)
	}

	isBlocked, err := st.IsBlocked(caller, target)
	if err != nil {
		logger.Error("user_favorite: check block status", "error", err)
	}
	if isBlocked {
		respondEphemeral(s, i, fmt.Sprintf(messages.FavoriteAddedStillBlocked, target))
	} else {
		respondEphemeral(s, i, fmt.Sprintf(messages.FavoriteAdded, target))
	}
}
