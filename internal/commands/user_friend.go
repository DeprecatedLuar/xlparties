package commands

import (
	"fmt"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/messages"
	"xlparties/internal/party"
	"xlparties/internal/store"
)

func handleUserFriend(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store, pm *party.Manager) {
	caller, target, ok := callerAndTarget(s, i)
	if !ok {
		return
	}

	alreadyFriend, err := st.IsFriend(caller, target)
	if err != nil {
		logger.Error("user_friend", "error", err)
		respondEphemeral(s, i, messages.FailedAddFriend)
		return
	}
	if alreadyFriend {
		respondEphemeral(s, i, fmt.Sprintf(messages.AlreadyFriend, target))
		return
	}

	if err := addFriend(s, i.GuildID, st, caller, target); err != nil {
		logger.Error("user_friend", "error", err)
		respondEphemeral(s, i, messages.FailedAddFriend)
		return
	}
	if err := pm.RewriteAffectedChannels(caller); err != nil {
		logger.Error("user_friend: rewrite affected channels", "caller", caller, "error", err)
	}

	isBlocked, err := st.IsBlocked(caller, target)
	if err != nil {
		logger.Error("user_friend: check block status", "error", err)
	}
	if isBlocked {
		respondEphemeral(s, i, fmt.Sprintf(messages.FriendAddedStillBlocked, target))
	} else {
		respondEphemeral(s, i, fmt.Sprintf(messages.FriendAdded, target))
	}
}

// addFriend writes the friend edge and best-effort DMs target that caller
// added them as a friend. Shared by handleUserFriend and handleUserFavorite,
// since favoriting a non-friend creates the friend edge first (favorite
// implies friend) and the same DM should fire either way.
func addFriend(s *discordgo.Session, guildID string, st *store.Store, caller, target int64) error {
	if err := st.UpsertFriend(caller, target); err != nil {
		return err
	}
	notifyFriendAdded(s, guildID, caller, target)
	return nil
}

// notifyFriendAdded best-effort DMs target that caller added them as a
// friend, with the command to reciprocate. DM failures (e.g. target has
// server DMs disabled) are logged, not surfaced to the caller.
func notifyFriendAdded(s *discordgo.Session, guildID string, caller, target int64) {
	channel, err := s.UserChannelCreate(fmt.Sprint(target))
	if err != nil {
		logger.Error("user_friend: could not open DM", "target", target, "error", err)
		return
	}

	guildName := guildID
	if guild, err := s.State.Guild(guildID); err == nil {
		guildName = guild.Name
	}

	msg := fmt.Sprintf(messages.FriendAddedNotif, messages.RandomGreeting(), caller, guildName, caller)
	if _, err := s.ChannelMessageSend(channel.ID, msg); err != nil {
		logger.Error("user_friend: could not DM", "target", target, "error", err)
	}
}
