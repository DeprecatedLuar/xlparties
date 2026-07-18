package commands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/messages"
	"xlparties/internal/party"
	"xlparties/internal/store"
)

// partyModeComponentPrefix namespaces the CustomID of the three access-mode
// buttons so route() can tell them apart from other components.
const partyModeComponentPrefix = "party_mode_"

// partyModeLabel is the human-readable name shown in prompts, buttons, and
// the confirmation message for each store.AccessMode* constant.
var partyModeLabel = map[string]string{
	store.AccessModeFriendsOfFriends: "Friends of friends",
	store.AccessModeFriendsOnly:      "Friends only",
	store.AccessModeInviteOnly:       "Invite only",
}

func handlePartyMode(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store, pm *party.Manager) {
	channelID, ok := ownedPartyChannel(s, i, st)
	if !ok {
		return
	}

	mode, given := modeOption(i)
	if !given {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    messages.PartyModePrompt,
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{partyModeButtonRow()},
			},
		})
		if err != nil {
			logger.Error("party_mode: respond with mode buttons", "error", err)
		}
		return
	}

	applyAccessMode(s, i, pm, channelID, mode, respondPublic)
}

func handlePartyModeComponent(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store, pm *party.Manager) {
	channelID, ok := ownedPartyChannel(s, i, st)
	if !ok {
		return
	}

	mode := strings.TrimPrefix(i.MessageComponentData().CustomID, partyModeComponentPrefix)
	if _, valid := partyModeLabel[mode]; !valid {
		logger.Warn("party_mode: unknown component custom id", "custom_id", i.MessageComponentData().CustomID)
		return
	}

	applyAccessMode(s, i, pm, channelID, mode, func(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    message,
				Components: []discordgo.MessageComponent{},
			},
		}); err != nil {
			logger.Error("party_mode: update component message", "error", err)
		}
	})
}

// applyAccessMode calls SetAccessMode and reports the result through
// respond, which differs between the slash-command path (a fresh public
// reply) and the button path (editing the ephemeral prompt in place).
func applyAccessMode(s *discordgo.Session, i *discordgo.InteractionCreate, pm *party.Manager, channelID int64, mode string, respond func(*discordgo.Session, *discordgo.InteractionCreate, string)) {
	if err := pm.SetAccessMode(channelID, mode); err != nil {
		logger.Error("party_mode: set access mode", "error", err)
		respond(s, i, fmt.Sprintf(messages.FailedSetPartyMode, mode))
		return
	}
	respond(s, i, fmt.Sprintf(messages.PartyModeSet, partyModeLabel[mode]))
}

// ownedPartyChannel resolves the interaction's channel as a party channel
// and verifies the caller is its owner, responding with an ephemeral error
// and returning ok=false otherwise.
func ownedPartyChannel(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store) (channelID int64, ok bool) {
	caller, err := callerID(i)
	if err != nil {
		logger.Error("party_mode: resolve caller id", "error", err)
		respondEphemeral(s, i, messages.FailedResolveCaller)
		return 0, false
	}

	channelID, err = strconv.ParseInt(i.ChannelID, 10, 64)
	if err != nil {
		logger.Error("party_mode: parse channel id", "error", err)
		respondEphemeral(s, i, messages.FailedResolveChannel)
		return 0, false
	}

	activeParty, found, err := st.PartyByChannel(channelID)
	if err != nil {
		logger.Error("party_mode: lookup party", "error", err)
		respondEphemeral(s, i, messages.FailedLookupParty)
		return 0, false
	}
	if !found {
		respondEphemeral(s, i, messages.NotInParty)
		return 0, false
	}
	if activeParty.OwnerID != caller {
		respondEphemeral(s, i, fmt.Sprintf(messages.MustBeOwner, activeParty.OwnerID))
		return 0, false
	}
	return channelID, true
}

func modeOption(i *discordgo.InteractionCreate) (string, bool) {
	for _, opt := range i.ApplicationCommandData().Options {
		if opt.Name == "mode" {
			return opt.StringValue(), true
		}
	}
	return "", false
}

func partyModeButtonRow() discordgo.ActionsRow {
	modes := []string{store.AccessModeFriendsOfFriends, store.AccessModeFriendsOnly, store.AccessModeInviteOnly}
	buttons := make([]discordgo.MessageComponent, 0, len(modes))
	for _, mode := range modes {
		buttons = append(buttons, discordgo.Button{
			Label:    partyModeLabel[mode],
			Style:    discordgo.PrimaryButton,
			CustomID: partyModeComponentPrefix + mode,
		})
	}
	return discordgo.ActionsRow{Components: buttons}
}
