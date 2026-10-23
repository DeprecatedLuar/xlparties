package commands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/messages"
)

// userOptionBaseName names the first user slot; further slots append their
// 1-based index.
const userOptionBaseName = "user"

// callerID returns the snowflake of the user who invoked the interaction.
// Commands are guild-scoped only, so the caller is always a guild member.
func callerID(i *discordgo.InteractionCreate) (int64, error) {
	return strconv.ParseInt(i.Member.User.ID, 10, 64)
}

// targetedCall is a resolved invocation of a command that acts on one or
// more users: the caller, the valid targets, and one reply line for each
// requested target that was skipped as invalid (self or the bot).
type targetedCall struct {
	caller  int64
	targets []int64
	skipped []string
}

// userOptionIDs returns the snowflakes of every user option, in option
// order, without duplicates.
func userOptionIDs(i *discordgo.InteractionCreate) ([]int64, error) {
	var ids []int64
	seen := make(map[int64]bool)
	for _, opt := range i.ApplicationCommandData().Options {
		if opt.Type != discordgo.ApplicationCommandOptionUser {
			continue
		}
		id, err := strconv.ParseInt(opt.UserValue(nil).ID, 10, 64)
		if err != nil {
			return nil, err
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("missing required option %q", userOptionName(1))
	}
	return ids, nil
}

// resolveTargetedCall resolves the caller and every user option, reporting
// the user-facing message for whichever check failed (if any) so the caller
// can respond however fits its own interaction flow. Self and bot targets
// are skipped with a reply line; the call fails only when no valid target
// remains.
func resolveTargetedCall(s *discordgo.Session, i *discordgo.InteractionCreate) (call targetedCall, failure string, ok bool) {
	caller, err := callerID(i)
	if err != nil {
		logger.Error("resolve caller id", "error", err)
		return targetedCall{}, messages.FailedResolveCaller, false
	}
	ids, err := userOptionIDs(i)
	if err != nil {
		logger.Error("resolve target user ids", "error", err)
		return targetedCall{}, messages.FailedResolveTarget, false
	}

	call.caller = caller
	for _, id := range ids {
		switch {
		case id == caller:
			call.skipped = append(call.skipped, messages.CannotTargetSelf)
		case strconv.FormatInt(id, 10) == s.State.User.ID:
			call.skipped = append(call.skipped, messages.NuhUh)
		default:
			call.targets = append(call.targets, id)
		}
	}
	if len(call.targets) == 0 {
		return targetedCall{}, strings.Join(call.skipped, "\n"), false
	}
	return call, "", true
}

// callerAndTargets is the resolveTargetedCall wrapper every handler that
// responds immediately (no deferred interaction) uses: it responds with an
// ephemeral error itself and returns ok=false on failure.
func callerAndTargets(s *discordgo.Session, i *discordgo.InteractionCreate) (call targetedCall, ok bool) {
	call, failure, ok := resolveTargetedCall(s, i)
	if !ok {
		respondEphemeral(s, i, failure)
	}
	return call, ok
}

// reply runs action for each target and joins the skipped-target lines and
// every action result into one reply.
func (c targetedCall) reply(action func(target int64) string) string {
	lines := append([]string(nil), c.skipped...)
	for _, target := range c.targets {
		lines = append(lines, action(target))
	}
	return strings.Join(lines, "\n")
}

// userOptionName is the slash-command option name for the nth user slot
// (1-based): "user", "user2", "user3", ...
func userOptionName(n int) string {
	if n == 1 {
		return userOptionBaseName
	}
	return userOptionBaseName + strconv.Itoa(n)
}

func respondEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	respond(s, i, message, discordgo.MessageFlagsEphemeral)
}

func respondPublic(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	respond(s, i, message, 0)
}

func respond(s *discordgo.Session, i *discordgo.InteractionCreate, message string, flags discordgo.MessageFlags) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Flags:   flags,
			// The bot runs with Administrator (mention-everyone included),
			// and replies like /party_info and /user_list render raw
			// user/@everyone mentions as data, not as pings. Suppress all
			// mention parsing rather than relying on ephemeral delivery
			// rules to keep those silent.
			AllowedMentions: &discordgo.MessageAllowedMentions{},
		},
	})
	if err != nil {
		logger.Error("respond to interaction", "interaction", i.ID, "error", err)
	}
}

// deferEphemeral acknowledges the interaction immediately with an ephemeral
// placeholder, buying up to 15 minutes to do slow work (multiple Discord
// REST calls, DMs) that would otherwise blow the 3-second ACK deadline and
// surface as Discord's "The application did not respond." Every response
// after this must go through editDeferred instead of
// respond/respondEphemeral/respondPublic, since the initial response slot
// is now spent.
func deferEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate) error {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		logger.Error("defer interaction response", "interaction", i.ID, "error", err)
	}
	return err
}

// editDeferred fills in a deferEphemeral placeholder. Only valid for a
// handler whose every response is ephemeral: a deferred response's own
// visibility is fixed at the ACK and can't be changed by editing it.
func editDeferred(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	edit := &discordgo.WebhookEdit{Content: &message, AllowedMentions: &discordgo.MessageAllowedMentions{}}
	if _, err := s.InteractionResponseEdit(i.Interaction, edit); err != nil {
		logger.Error("edit deferred interaction response", "interaction", i.ID, "error", err)
	}
}

// userInVoiceChannel reports whether userID is currently connected to
// channelID according to the cached guild voice states.
func userInVoiceChannel(s *discordgo.Session, guildID, channelID, userID string) (bool, error) {
	guild, err := s.State.Guild(guildID)
	if err != nil {
		return false, err
	}
	for _, vs := range guild.VoiceStates {
		if vs.UserID == userID && vs.ChannelID == channelID {
			return true, nil
		}
	}
	return false, nil
}
