// Package commands defines the bot's slash commands, registers them
// guild-scoped, and routes interactions to their handlers.
package commands

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"

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
		Name:        "block",
		Description: "Block a user from your party by default",
		Options:     []*discordgo.ApplicationCommandOption{userOption("The user to block")},
	},
	{
		Name:        "unblock",
		Description: "Unblock a user",
		Options:     []*discordgo.ApplicationCommandOption{userOption("The user to unblock")},
	},
	{
		Name:        "vc_allow",
		Description: "Allow a user into your current party, overriding any default",
		Options:     []*discordgo.ApplicationCommandOption{userOption("The user to allow")},
	},
	{
		Name:        "vc_deny",
		Description: "Deny a user from your current party, overriding any default",
		Options:     []*discordgo.ApplicationCommandOption{userOption("The user to deny")},
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
	"block":         handleBlock,
	"unblock":       handleUnblock,
	"vc_allow":      handleVCAllow,
	"vc_deny":       handleVCDeny,
	"configure":     handleConfigure,
}

// Register creates every command guild-scoped and wires interaction routing.
// Returns the created commands so the caller can hold onto them if needed.
func Register(s *discordgo.Session, guildID string, st *store.Store) ([]*discordgo.ApplicationCommand, error) {
	s.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		route(s, i, st)
	})

	created := make([]*discordgo.ApplicationCommand, 0, len(specs))
	for _, spec := range specs {
		cmd, err := s.ApplicationCommandCreate(s.State.User.ID, guildID, spec)
		if err != nil {
			return nil, fmt.Errorf("register command %q: %w", spec.Name, err)
		}
		created = append(created, cmd)
	}
	return created, nil
}

func route(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}
	name := i.ApplicationCommandData().Name
	handler, ok := handlers[name]
	if !ok {
		log.Printf("unknown command interaction: %q", name)
		return
	}
	handler(s, i, st)
}
