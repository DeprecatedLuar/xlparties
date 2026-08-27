package commands

import (
	"fmt"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/messages"
	"xlparties/internal/store"
)

// partyPresetComponentPrefix namespaces the CustomID of the preset select
// menu so route() can tell it apart from other components.
const partyPresetComponentPrefix = "party_preset_"

// partyPresetClearValue is the select option that removes the caller's
// saved preset, distinct from any store.AccessMode* constant.
const partyPresetClearValue = "clear"

func handlePartyPreset(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store) {
	caller, err := callerID(i)
	if err != nil {
		logger.Error("party_preset: resolve caller id", "error", err)
		respondEphemeral(s, i, messages.FailedResolveCaller)
		return
	}

	mode, given := presetModeOption(i)
	if !given {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    messages.PartyPresetPrompt,
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{partyPresetSelectRow()},
			},
		})
		if err != nil {
			logger.Error("party_preset: respond with select", "error", err)
		}
		return
	}

	applyPreset(s, i, st, caller, mode, respondEphemeral)
}

func handlePartyPresetComponent(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store) {
	caller, err := callerID(i)
	if err != nil {
		logger.Error("party_preset: resolve caller id", "error", err)
		respondEphemeral(s, i, messages.FailedResolveCaller)
		return
	}

	values := i.MessageComponentData().Values
	if len(values) != 1 {
		logger.Warn("party_preset: unexpected select values", "values", values)
		return
	}
	mode := values[0]
	if mode != partyPresetClearValue {
		if _, valid := partyModeLabel[mode]; !valid {
			logger.Warn("party_preset: unknown select value", "value", mode)
			return
		}
	}

	applyPreset(s, i, st, caller, mode, func(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
		if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    message,
				Components: []discordgo.MessageComponent{},
			},
		}); err != nil {
			logger.Error("party_preset: update component message", "error", err)
		}
	})
}

// applyPreset writes or clears caller's preset and reports the result
// through respond, which differs between the slash-command path (a fresh
// ephemeral reply) and the select path (editing the ephemeral prompt in
// place).
func applyPreset(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store, caller int64, mode string, respond func(*discordgo.Session, *discordgo.InteractionCreate, string)) {
	if mode == partyPresetClearValue {
		if err := st.DeletePreset(caller); err != nil {
			logger.Error("party_preset: clear preset", "error", err)
			respond(s, i, messages.FailedClearPreset)
			return
		}
		respond(s, i, fmt.Sprintf(messages.PartyPresetCleared, partyModeLabel[store.DefaultAccessMode]))
		return
	}

	if err := st.UpsertPreset(caller, mode); err != nil {
		logger.Error("party_preset: set preset", "error", err)
		respond(s, i, messages.FailedSetPartyPreset)
		return
	}
	respond(s, i, fmt.Sprintf(messages.PartyPresetSet, partyModeLabel[mode]))
}

func presetModeOption(i *discordgo.InteractionCreate) (string, bool) {
	for _, opt := range i.ApplicationCommandData().Options {
		if opt.Name == "mode" {
			return opt.StringValue(), true
		}
	}
	return "", false
}

func partyPresetSelectRow() discordgo.ActionsRow {
	modes := []string{store.AccessModeFriendsOfFriends, store.AccessModeFriendsOnly, store.AccessModeInviteOnly, store.AccessModePublic}
	options := make([]discordgo.SelectMenuOption, 0, len(modes)+1)
	for _, mode := range modes {
		options = append(options, discordgo.SelectMenuOption{
			Label: partyModeLabel[mode],
			Value: mode,
		})
	}
	options = append(options, discordgo.SelectMenuOption{
		Label: "Clear (use default)",
		Value: partyPresetClearValue,
	})
	return discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.SelectMenu{
				CustomID: partyPresetComponentPrefix,
				Options:  options,
			},
		},
	}
}
