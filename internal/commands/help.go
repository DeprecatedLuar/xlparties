package commands

import (
	"github.com/bwmarrin/discordgo"

	"xlparties/internal/store"
)

const helpText = `**xlparties commands**
` + "`/friend_add user`" + ` — add a friend, granting them default access to your party
` + "`/friend_remove user`" + ` — remove a friend
` + "`/friend_list`" + ` — list your friends
` + "`/enemy_add user`" + ` — add an enemy, blocking them from your party by default
` + "`/enemy_remove user`" + ` — remove an enemy
` + "`/enemy_list`" + ` — list your enemies
` + "`/party_allow user`" + ` — allow a user into your current party (overrides defaults)
` + "`/party_block user`" + ` — block a user from your current party (overrides defaults)
` + "`/party_kick user`" + ` — kick a user from your current party voice channel
` + "`/party_ban user`" + ` — ban a user from your current party (deny access + kick if present)
` + "`/party_invite user`" + ` — invite anyone to your current party; access is tied to their presence
` + "`/party_mode [mode]`" + ` — view or set your current party's access mode (friends of friends / friends only / invite only)
` + "`/configure`" + ` — (admin) set the watch channel and party category
` + "`/help`" + ` — show this message

Join the watch channel to spawn your own private party channel.`

func handleHelp(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store) {
	respondEphemeral(s, i, helpText)
}
