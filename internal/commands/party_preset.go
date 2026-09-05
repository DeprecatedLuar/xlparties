package commands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/messages"
	"xlparties/internal/store"
)

// partyPresetComponentPrefix namespaces the CustomID of the mode select
// menu so route() can tell it apart from other components.
const partyPresetComponentPrefix = "party_preset_"

// partyPresetLimitComponentPrefix namespaces the CustomID of the limit
// select menu. Checked before partyPresetComponentPrefix in route() since it
// shares that prefix.
const partyPresetLimitComponentPrefix = "party_preset_limit_"

// partyPresetClearValue is the select option that removes the caller's
// saved preset, distinct from any store.AccessMode* constant.
const partyPresetClearValue = "clear"

// partyPresetLimitUnlimitedValue is the limit select option that resets the
// saved preset limit back to 0 (unlimited).
const partyPresetLimitUnlimitedValue = "unlimited"

// partyPresetLimitChoices lists the limit select's options, in order.
var partyPresetLimitChoices = []int{2, 4, 6, 8, 10}

func handlePartyPreset(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store) {
	caller, err := callerID(i)
	if err != nil {
		logger.Error("party_preset: resolve caller id", "error", err)
		respondEphemeral(s, i, messages.FailedResolveCaller)
		return
	}

	mode, modeGiven, limit, limitGiven := presetOptions(i)
	if !modeGiven && !limitGiven {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content:    messages.PartyPresetPrompt,
				Flags:      discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{partyPresetSelectRow(), partyPresetLimitSelectRow()},
			},
		})
		if err != nil {
			logger.Error("party_preset: respond with select", "error", err)
		}
		return
	}

	var lines []string
	if modeGiven {
		lines = append(lines, applyPresetMode(st, caller, mode))
	}
	if limitGiven {
		lines = append(lines, applyPresetLimit(st, caller, limit))
	}
	respondEphemeral(s, i, strings.Join(lines, "\n"))
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

	message := applyPresetMode(st, caller, mode)
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    message,
			Components: []discordgo.MessageComponent{},
		},
	}); err != nil {
		logger.Error("party_preset: update component message", "error", err)
	}
}

func handlePartyPresetLimitComponent(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store) {
	caller, err := callerID(i)
	if err != nil {
		logger.Error("party_preset: resolve caller id", "error", err)
		respondEphemeral(s, i, messages.FailedResolveCaller)
		return
	}

	values := i.MessageComponentData().Values
	if len(values) != 1 {
		logger.Warn("party_preset: unexpected limit select values", "values", values)
		return
	}

	limit := 0
	if values[0] != partyPresetLimitUnlimitedValue {
		limit, err = strconv.Atoi(values[0])
		if err != nil {
			logger.Warn("party_preset: unknown limit select value", "value", values[0])
			return
		}
	}

	message := applyPresetLimit(st, caller, limit)
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    message,
			Components: []discordgo.MessageComponent{},
		},
	}); err != nil {
		logger.Error("party_preset: update limit component message", "error", err)
	}
}

// applyPresetMode writes or clears caller's mode preset and returns the
// message describing the result.
func applyPresetMode(st *store.Store, caller int64, mode string) string {
	if mode == partyPresetClearValue {
		if err := st.DeletePreset(caller); err != nil {
			logger.Error("party_preset: clear preset", "error", err)
			return messages.FailedClearPreset
		}
		return fmt.Sprintf(messages.PartyPresetCleared, partyModeLabel[store.DefaultAccessMode])
	}

	if err := st.UpsertPreset(caller, mode); err != nil {
		logger.Error("party_preset: set preset", "error", err)
		return messages.FailedSetPartyPreset
	}
	return fmt.Sprintf(messages.PartyPresetSet, partyModeLabel[mode])
}

// applyPresetLimit writes caller's limit preset and returns the message
// describing the result. limit 0 means unlimited.
func applyPresetLimit(st *store.Store, caller int64, limit int) string {
	if err := st.UpsertPresetLimit(caller, limit); err != nil {
		logger.Error("party_preset: set preset limit", "error", err)
		return messages.FailedSetPartyPresetLimit
	}
	if limit == 0 {
		return messages.PartyPresetLimitCleared
	}
	return fmt.Sprintf(messages.PartyPresetLimitSet, limit)
}

func presetOptions(i *discordgo.InteractionCreate) (mode string, modeGiven bool, limit int, limitGiven bool) {
	for _, opt := range i.ApplicationCommandData().Options {
		switch opt.Name {
		case "mode":
			mode, modeGiven = opt.StringValue(), true
		case "limit":
			limit, limitGiven = int(opt.IntValue()), true
		}
	}
	return mode, modeGiven, limit, limitGiven
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
				CustomID:    partyPresetComponentPrefix,
				Placeholder: "Default access mode",
				Options:     options,
			},
		},
	}
}

func partyPresetLimitSelectRow() discordgo.ActionsRow {
	options := make([]discordgo.SelectMenuOption, 0, len(partyPresetLimitChoices)+1)
	for _, limit := range partyPresetLimitChoices {
		options = append(options, discordgo.SelectMenuOption{
			Label: fmt.Sprintf("%d", limit),
			Value: strconv.Itoa(limit),
		})
	}
	options = append(options, discordgo.SelectMenuOption{
		Label: "Unlimited",
		Value: partyPresetLimitUnlimitedValue,
	})
	return discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			discordgo.SelectMenu{
				CustomID:    partyPresetLimitComponentPrefix,
				Placeholder: "Default user limit",
				Options:     options,
			},
		},
	}
}
