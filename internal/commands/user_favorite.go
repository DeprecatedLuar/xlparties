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
	runUserEdit(s, i, pm, "user_favorite", func(caller, target int64) string {
		alreadyFavorite, err := st.IsFavorite(caller, target)
		if err != nil {
			logger.Error("user_favorite", "error", err)
			return messages.FailedAddFavorite
		}
		if alreadyFavorite {
			return fmt.Sprintf(messages.AlreadyFavorite, target)
		}

		isFriend, err := st.IsFriend(caller, target)
		if err != nil {
			logger.Error("user_favorite: check friend status", "error", err)
			return messages.FailedAddFavorite
		}
		if !isFriend {
			if err := addFriend(s, i.GuildID, st, caller, target); err != nil {
				logger.Error("user_favorite: add friend", "error", err)
				return messages.FailedAddFavorite
			}
		}

		if err := st.UpsertFavorite(caller, target); err != nil {
			logger.Error("user_favorite", "error", err)
			return messages.FailedAddFavorite
		}

		isBlocked, err := st.IsBlocked(caller, target)
		if err != nil {
			logger.Error("user_favorite: check block status", "error", err)
		}
		if isBlocked {
			return fmt.Sprintf(messages.FavoriteAddedStillBlocked, target)
		}
		return fmt.Sprintf(messages.FavoriteAdded, target)
	})
}
