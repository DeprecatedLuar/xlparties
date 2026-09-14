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

func handlePartyInfo(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store) {
	caller, err := callerID(i)
	if err != nil {
		logger.Error("party_info: resolve caller id", "error", err)
		respondEphemeral(s, i, messages.FailedResolveCaller)
		return
	}

	channelID, err := strconv.ParseInt(i.ChannelID, 10, 64)
	if err != nil {
		logger.Error("party_info: parse channel id", "error", err)
		respondEphemeral(s, i, messages.FailedResolveChannel)
		return
	}

	activeParty, found, err := st.PartyByChannel(channelID)
	if err != nil {
		logger.Error("party_info: lookup party", "error", err)
		respondEphemeral(s, i, messages.FailedLookupParty)
		return
	}
	if !found {
		respondEphemeral(s, i, messages.NotInParty)
		return
	}

	channel, err := s.Channel(i.ChannelID)
	if err != nil {
		logger.Error("party_info: fetch channel", "error", err)
		respondEphemeral(s, i, messages.FailedLookupParty)
		return
	}
	limitDisplay := messages.PartyInfoNoLimit
	if channel.UserLimit != 0 {
		limitDisplay = strconv.Itoa(channel.UserLimit)
	}

	allowedLines, blockedLines := partitionOverwrites(channel.PermissionOverwrites, i.GuildID, s.State.User.ID)

	preset, found, err := st.PresetForUser(caller)
	if err != nil {
		logger.Error("party_info: lookup preset", "error", err)
		respondEphemeral(s, i, messages.FailedLookupParty)
		return
	}
	presetLine := fmt.Sprintf(messages.NoPartyPreset, partyModeLabel[store.DefaultAccessMode])
	if found {
		presetLine = fmt.Sprintf(messages.PartyPresetCurrent, partyModeLabel[preset])
	}

	presetLimit, found, err := st.PresetLimitForUser(caller)
	if err != nil {
		logger.Error("party_info: lookup preset limit", "error", err)
		respondEphemeral(s, i, messages.FailedLookupParty)
		return
	}
	presetLimitLine := messages.NoPartyPresetLimit
	if found && presetLimit != 0 {
		presetLimitLine = fmt.Sprintf(messages.PartyPresetLimitCurrent, presetLimit)
	}

	respondEphemeral(s, i, fmt.Sprintf(messages.PartyInfoHeader,
		partyModeLabel[activeParty.AccessMode],
		limitDisplay,
		overrideList(allowedLines),
		overrideList(blockedLines),
		presetLine,
		presetLimitLine,
	))
}

// partitionOverwrites mirrors the channel's actual permission overwrites
// into allowed/blocked mention lines, rather than recomputing access from
// store state - the channel overwrites are the enforced access model (see
// buildRewriteOverwrites), so reading them back is the only rendering that
// can't drift from what Discord actually grants. botUserID is skipped: the
// bot's own self-allow overwrite is implementation detail, not a party
// access decision.
func partitionOverwrites(overwrites []*discordgo.PermissionOverwrite, guildID, botUserID string) (allowed, blocked []string) {
	for _, ow := range overwrites {
		switch ow.Type {
		case discordgo.PermissionOverwriteTypeMember:
			if ow.ID == botUserID {
				continue
			}
			mention := fmt.Sprintf("<@%s>", ow.ID)
			if ow.Allow&party.PartyChannelPermissions == party.PartyChannelPermissions {
				allowed = append(allowed, mention)
			} else if ow.Deny&party.PartyChannelPermissions == party.PartyChannelPermissions {
				blocked = append(blocked, mention)
			}
		case discordgo.PermissionOverwriteTypeRole:
			mention := fmt.Sprintf("<@&%s>", ow.ID)
			if ow.ID == guildID {
				mention = messages.EveryoneMention
			}
			if ow.Allow&party.PartyChannelPermissions == party.PartyChannelPermissions {
				allowed = append(allowed, mention)
			} else if ow.Deny&party.PartyChannelPermissions == party.PartyChannelPermissions {
				blocked = append(blocked, mention)
			}
		}
	}
	return allowed, blocked
}

func overrideList(lines []string) string {
	if len(lines) == 0 {
		return messages.NoOverrides
	}
	return strings.Join(lines, "\n")
}
