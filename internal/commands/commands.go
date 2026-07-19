// Package commands defines the bot's slash commands, registers them
// guild-scoped, and routes interactions to their handlers.
package commands

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/party"
	"xlparties/internal/store"
)

var manageGuildPermission = int64(discordgo.PermissionManageGuild)

var specs = []*discordgo.ApplicationCommand{
	{
		Name:        "friend_add",
		Description: "Add a user as a friend, granting them default access to your party",
		Options:     []*discordgo.ApplicationCommandOption{userOption("The user to add as a friend")},
	},
	{
		Name:        "friend_remove",
		Description: "Remove a user as a friend",
		Options:     []*discordgo.ApplicationCommandOption{userOption("The user to remove as a friend")},
	},
	{
		Name:        "friend_list",
		Description: "List your friends",
	},
	{
		Name:        "enemy_add",
		Description: "Add a user as an enemy, blocking them from your party by default",
		Options:     []*discordgo.ApplicationCommandOption{userOption("The user to add as an enemy")},
	},
	{
		Name:        "enemy_remove",
		Description: "Remove a user as an enemy",
		Options:     []*discordgo.ApplicationCommandOption{userOption("The user to remove as an enemy")},
	},
	{
		Name:        "enemy_list",
		Description: "List your enemies",
	},
	{
		Name:        "party_allow",
		Description: "Allow a user into your current party, overriding any default",
		Options:     []*discordgo.ApplicationCommandOption{userOption("The user to allow")},
	},
	{
		Name:        "party_block",
		Description: "Block a user from your current party, overriding any default",
		Options:     []*discordgo.ApplicationCommandOption{userOption("The user to block")},
	},
	{
		Name:        "party_kick",
		Description: "Kick a user from your current party voice channel",
		Options:     []*discordgo.ApplicationCommandOption{userOption("The user to kick")},
	},
	{
		Name:        "party_ban",
		Description: "Ban a user from your current party (denies access and kicks them if present)",
		Options:     []*discordgo.ApplicationCommandOption{userOption("The user to ban")},
	},
	{
		Name:        "party_mode",
		Description: "View or set your current party's access mode",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "mode",
				Description: "The access mode to switch to",
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{Name: "Friends of friends", Value: store.AccessModeFriendsOfFriends},
					{Name: "Friends only", Value: store.AccessModeFriendsOnly},
					{Name: "Invite only", Value: store.AccessModeInviteOnly},
				},
			},
		},
	},
	{
		Name:        "help",
		Description: "List available commands",
	},
	{
		Name:                     "configure",
		Description:              "Configure bot settings for this server",
		DefaultMemberPermissions: &manageGuildPermission,
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "watch_channel",
				Description: "Set the voice channel that triggers party creation",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:         discordgo.ApplicationCommandOptionChannel,
						Name:         "channel",
						Description:  "The voice channel to watch",
						Required:     true,
						ChannelTypes: []discordgo.ChannelType{discordgo.ChannelTypeGuildVoice},
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "category",
				Description: "Set the category new party channels spawn under",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:         discordgo.ApplicationCommandOptionChannel,
						Name:         "category",
						Description:  "The category for new party channels",
						Required:     true,
						ChannelTypes: []discordgo.ChannelType{discordgo.ChannelTypeGuildCategory},
					},
				},
			},
		},
	},
}

func userOption(description string) *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionUser,
		Name:        "user",
		Description: description,
		Required:    true,
	}
}

type handlerFunc func(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store)

var handlers = map[string]handlerFunc{
	"friend_add":    handleFriendAdd,
	"friend_remove": handleFriendRemove,
	"friend_list":   handleFriendList,
	"enemy_add":     handleEnemyAdd,
	"enemy_remove":  handleEnemyRemove,
	"enemy_list":    handleEnemyList,
	"party_allow":   handlePartyAllow,
	"party_block":   handlePartyBlock,
	"party_kick":    handlePartyKick,
	"party_ban":     handlePartyBan,
	"configure":     handleConfigure,
	"help":          handleHelp,
}

// Register creates every command guild-scoped and wires interaction routing.
// Returns the created commands so the caller can hold onto them if needed.
// partyManager is only used by /party_mode, which is why it isn't threaded
// through the shared handlerFunc signature the way *store.Store is.
func Register(s *discordgo.Session, guildID string, st *store.Store, partyManager *party.Manager) ([]*discordgo.ApplicationCommand, error) {
	s.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		route(s, i, st, partyManager)
	})

	created, err := s.ApplicationCommandBulkOverwrite(s.State.User.ID, guildID, specs)
	if err != nil {
		return nil, fmt.Errorf("register commands: %w", err)
	}
	return created, nil
}

func route(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store, partyManager *party.Manager) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		name := i.ApplicationCommandData().Name
		logger.Info("command invoked", "command", name, "caller", i.Member.User.ID)
		if name == "party_mode" {
			handlePartyMode(s, i, st, partyManager)
			return
		}
		handler, ok := handlers[name]
		if !ok {
			logger.Warn("unknown command interaction", "command", name)
			return
		}
		handler(s, i, st)
	case discordgo.InteractionMessageComponent:
		customID := i.MessageComponentData().CustomID
		if strings.HasPrefix(customID, partyModeComponentPrefix) {
			handlePartyModeComponent(s, i, st, partyManager)
			return
		}
		logger.Warn("unknown component interaction", "custom_id", customID)
	}
}
