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

// maxTargets is how many users a multi-target command accepts per call.
const maxTargets = 5

var manageGuildPermission = int64(discordgo.PermissionManageGuild)

// limitOptionMin/limitOptionMax mirror internal/party's minUserLimit/
// maxUserLimit (Discord's own voice channel user_limit bounds), enforced
// here too so the client-side slash-command UI rejects out-of-range input
// before it reaches the bot.
var limitOptionMin = 0.0

const limitOptionMax = 99.0

var specs = []*discordgo.ApplicationCommand{
	{
		Name:        "user_friend",
		Description: "Add a user as a friend, granting them default access to your party",
		Options:     userOptions("The user to add as a friend"),
	},
	{
		Name:        "user_unfriend",
		Description: "Remove a user as a friend",
		Options:     userOptions("The user to remove as a friend"),
	},
	{
		Name:        "user_block",
		Description: "Add a user as an enemy, blocking them from your party by default",
		Options:     userOptions("The user to add as an enemy"),
	},
	{
		Name:        "user_unblock",
		Description: "Remove a user as an enemy",
		Options:     userOptions("The user to remove as an enemy"),
	},
	{
		Name:        "user_favorite",
		Description: "Add a user as a bestie, granting them default access to your besties-only party",
		Options:     userOptions("The user to add as a bestie"),
	},
	{
		Name:        "user_unfavorite",
		Description: "Remove a user as a bestie (they remain a friend)",
		Options:     userOptions("The user to remove as a bestie"),
	},
	{
		Name:        "user_list",
		Description: "List your besties, friends, enemies, and frenemies",
	},
	{
		Name:        "party_allow",
		Description: "Allow a user into your current party, overriding any default",
		Options:     userOptions("The user to allow"),
	},
	{
		Name:        "party_block",
		Description: "Block a user from your current party, overriding any default",
		Options:     userOptions("The user to block"),
	},
	{
		Name:        "party_kick",
		Description: "Kick a user from your current party voice channel",
		Options:     userOptions("The user to kick"),
	},
	{
		Name:        "party_ban",
		Description: "Ban a user from your current party (denies access and kicks them if present)",
		Options:     userOptions("The user to ban"),
	},
	{
		Name:        "party_invite",
		Description: "Invite a user to your current party; access lasts only while they stay connected",
		Options:     userOptions("The user to invite"),
	},
	{
		Name:        "party_create",
		Description: "Create your party now instead of joining the watch channel, optionally setting mode and limit inline",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "mode",
				Description: "Access mode for this party (defaults to your saved preset)",
				Choices:     accessModeChoices(),
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "limit",
				Description: "User limit for this party, 0-99 (defaults to your saved preset)",
				MinValue:    &limitOptionMin,
				MaxValue:    limitOptionMax,
			},
		},
	},
	{
		Name:        "party_mode",
		Description: "View or set your current party's access mode",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "mode",
				Description: "The access mode to switch to",
				Choices:     accessModeChoices(),
			},
		},
	},
	{
		Name:        "party_info",
		Description: "Show this party's type and manual allow/block overrides",
	},
	{
		Name:        "party_limit",
		Description: "Set your current party's voice channel user limit",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "limit",
				Description: "User limit, 0-99 (0 = unlimited)",
				Required:    true,
				MinValue:    &limitOptionMin,
				MaxValue:    limitOptionMax,
			},
		},
	},
	{
		Name:        "party_preset",
		Description: "View or set your saved default access mode and user limit, applied only when you next create a party",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "mode",
				Description: "The default access mode to save",
				Choices:     append(accessModeChoices(), &discordgo.ApplicationCommandOptionChoice{Name: "Clear (use default)", Value: partyPresetClearValue}),
			},
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "limit",
				Description: "Default user limit, 0-99 (0 = unlimited)",
				MinValue:    &limitOptionMin,
				MaxValue:    limitOptionMax,
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

// accessModeChoices builds the slash-command Choices list for every
// store.AccessMode* constant, in store.AccessModes display order, labeled
// via partyModeLabel. Shared by party_create, party_mode, and party_preset
// so the mode list is defined once instead of copied per command.
func accessModeChoices() []*discordgo.ApplicationCommandOptionChoice {
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(store.AccessModes))
	for _, mode := range store.AccessModes {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{Name: partyModeLabel[mode], Value: mode})
	}
	return choices
}

// userOptions builds the user slots every multi-target command takes: the
// first is required, the rest optional.
func userOptions(description string) []*discordgo.ApplicationCommandOption {
	options := make([]*discordgo.ApplicationCommandOption, 0, maxTargets)
	for n := 1; n <= maxTargets; n++ {
		options = append(options, &discordgo.ApplicationCommandOption{
			Type:        discordgo.ApplicationCommandOptionUser,
			Name:        userOptionName(n),
			Description: description,
			Required:    n == 1,
		})
	}
	return options
}

type handlerFunc func(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store, pm *party.Manager)

// storeOnly adapts a handler that needs no party manager to handlerFunc.
func storeOnly(h func(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store)) handlerFunc {
	return func(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store, _ *party.Manager) {
		h(s, i, st)
	}
}

var handlers = map[string]handlerFunc{
	"user_friend":     handleUserFriend,
	"user_unfriend":   handleUserUnfriend,
	"user_block":      handleUserBlock,
	"user_unblock":    handleUserUnblock,
	"user_favorite":   handleUserFavorite,
	"user_unfavorite": handleUserUnfavorite,
	"user_list":       storeOnly(handleUserList),
	"party_create":    handlePartyCreate,
	"party_mode":      handlePartyMode,
	"party_limit":     handlePartyLimit,
	"party_invite":    handlePartyInvite,
	"party_allow":     handlePartyAllow,
	"party_block":     handlePartyBlock,
	"party_kick":      handlePartyKick,
	"party_ban":       handlePartyBan,
	"party_info":      storeOnly(handlePartyInfo),
	"party_preset":    storeOnly(handlePartyPreset),
	"configure":       storeOnly(handleConfigure),
	"help":            storeOnly(handleHelp),
}

// Register creates every command guild-scoped and wires interaction routing.
// Returns the created commands so the caller can hold onto them if needed.
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
		handler, ok := handlers[name]
		if !ok {
			logger.Warn("unknown command interaction", "command", name)
			return
		}
		handler(s, i, st, partyManager)
	case discordgo.InteractionMessageComponent:
		customID := i.MessageComponentData().CustomID
		if strings.HasPrefix(customID, partyModeComponentPrefix) {
			handlePartyModeComponent(s, i, st, partyManager)
			return
		}
		if strings.HasPrefix(customID, partyPresetLimitComponentPrefix) {
			handlePartyPresetLimitComponent(s, i, st)
			return
		}
		if strings.HasPrefix(customID, partyPresetComponentPrefix) {
			handlePartyPresetComponent(s, i, st)
			return
		}
		logger.Warn("unknown component interaction", "custom_id", customID)
	}
}
