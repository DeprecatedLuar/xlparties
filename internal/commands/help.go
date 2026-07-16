package commands

import (
	"github.com/bwmarrin/discordgo"

	"xlparties/internal/store"
)

const helpText = `**xlparties commands**
` + "`/friend_add user`" + ` — add a friend, granting them default access to your party
` + "`/friend_remove user`" + ` — remove a friend
` + "`/block user`" + ` — block a user from your party by default
` + "`/unblock user`" + ` — unblock a user
` + "`/vc_allow user`" + ` — allow a user into your current party (overrides defaults)
` + "`/vc_deny user`" + ` — deny a user from your current party (overrides defaults)
` + "`/configure`" + ` — (admin) set the watch channel and party category
` + "`/help`" + ` — show this message

Join the watch channel to spawn your own private party channel.`

func handleHelp(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store) {
	respondEphemeral(s, i, helpText)
}
