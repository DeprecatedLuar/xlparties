package commands

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/store"
)

func handleFriendAdd(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store) {
	caller, target, ok := callerAndTarget(s, i)
	if !ok {
		return
	}

	alreadyFriend, err := st.IsFriend(caller, target)
	if err != nil {
		log.Printf("friend_add: %v", err)
		respondEphemeral(s, i, "failed to add friend")
		return
	}
	if alreadyFriend {
		respondEphemeral(s, i, fmt.Sprintf("<@%d> is already a friend", target))
		return
	}

	if err := st.UpsertFriend(caller, target); err != nil {
		log.Printf("friend_add: %v", err)
		respondEphemeral(s, i, "failed to add friend")
		return
	}
	respondEphemeral(s, i, fmt.Sprintf("added <@%d> as a friend", target))
	notifyFriendAdded(s, i.GuildID, caller, target)
}

// notifyFriendAdded best-effort DMs target that caller added them as a
// friend, with the command to reciprocate. DM failures (e.g. target has
// server DMs disabled) are logged, not surfaced to the caller.
func notifyFriendAdded(s *discordgo.Session, guildID string, caller, target int64) {
	channel, err := s.UserChannelCreate(fmt.Sprint(target))
	if err != nil {
		log.Printf("friend_add: could not open DM with %d: %v", target, err)
		return
	}

	guildName := guildID
	if guild, err := s.State.Guild(guildID); err == nil {
		guildName = guild.Name
	}

	msg := fmt.Sprintf("<@%d> added you as a friend in **%s**. In the server, run `/friend_add` and pick <@%d> as the user to add them back.", caller, guildName, caller)
	if _, err := s.ChannelMessageSend(channel.ID, msg); err != nil {
		log.Printf("friend_add: could not DM %d: %v", target, err)
	}
}
