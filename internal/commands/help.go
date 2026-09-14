package commands

import (
	"github.com/bwmarrin/discordgo"

	"xlparties/internal/store"
)

const helpText = `**xlparties commands**

**Relationships**
` + "`/user_friend user`" + ` — add a friend, granting them default access to your party
` + "`/user_unfriend user`" + ` — remove a friend (also removes bestie status)
` + "`/user_block user`" + ` — add an enemy, blocking them from your party by default
` + "`/user_unblock user`" + ` — remove an enemy
` + "`/user_favorite user`" + ` — add a bestie (implies friend), granting them default access to your besties-only party
` + "`/user_unfavorite user`" + ` — remove a bestie (they remain a friend)
` + "`/user_list`" + ` — list your besties, friends, enemies, and frenemies
_Friend, bestie, and enemy status are independent: blocking a friend (or friending an enemy) makes them a frenemy, and a block always wins over access._

**Your Party**
` + "`/party_create [mode] [limit]`" + ` — create your party now instead of joining the watch channel, optionally setting mode/limit inline (falls back to your saved preset for anything omitted)
` + "`/party_preset [mode] [limit]`" + ` — view or set your saved default access mode and user limit, applied only when you next create a party (never rewrites an existing one)
` + "`/party_allow user`" + ` — allow a user into your current party (overrides defaults)
` + "`/party_block user`" + ` — block a user from your current party (overrides defaults)
` + "`/party_kick user`" + ` — kick a user from your current party voice channel
` + "`/party_ban user`" + ` — ban a user from your current party (deny access + kick if present)
` + "`/party_invite user`" + ` — invite anyone to your current party; access is tied to their presence
` + "`/party_mode [mode]`" + ` — view or set your current party's access mode (friends of friends / friends only / besties only / invite only / public)
` + "`/party_limit limit`" + ` — set your current party's voice channel user limit (0-99, 0 = unlimited)
` + "`/party_info`" + ` — show your current party's access mode, user limit, allowed users, and blocked users

**Setup**
` + "`/configure`" + ` — (admin) set the watch channel and party category
` + "`/help`" + ` — show this message

Join the watch channel to spawn your own private party channel.`

func handleHelp(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store) {
	respondEphemeral(s, i, helpText)
}
