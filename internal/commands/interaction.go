package commands

import (
	"fmt"
	"strconv"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/messages"
)

// callerID returns the snowflake of the user who invoked the interaction.
// Commands are guild-scoped only, so the caller is always a guild member.
func callerID(i *discordgo.InteractionCreate) (int64, error) {
	return strconv.ParseInt(i.Member.User.ID, 10, 64)
}

// userOptionID returns the snowflake of the "user" option's value.
func userOptionID(i *discordgo.InteractionCreate) (int64, error) {
	data := i.ApplicationCommandData()
	for _, opt := range data.Options {
		if opt.Name == "user" {
			return strconv.ParseInt(opt.UserValue(nil).ID, 10, 64)
		}
	}
	return 0, fmt.Errorf("missing required option %q", "user")
}

// resolveCallerAndTarget resolves the caller and the "user" option,
// reporting the user-facing message for whichever check failed (if any) so
// the caller can respond however fits its own interaction flow, instead of
// this function assuming a plain ephemeral respond.
func resolveCallerAndTarget(s *discordgo.Session, i *discordgo.InteractionCreate) (caller, target int64, failure string, ok bool) {
	caller, err := callerID(i)
	if err != nil {
		logger.Error("resolve caller id", "error", err)
		return 0, 0, messages.FailedResolveCaller, false
	}
	target, err = userOptionID(i)
	if err != nil {
		logger.Error("resolve target user id", "error", err)
		return 0, 0, messages.FailedResolveTarget, false
	}
	if target == caller {
		return 0, 0, messages.CannotTargetSelf, false
	}
	if strconv.FormatInt(target, 10) == s.State.User.ID {
		return 0, 0, messages.NuhUh, false
	}
	return caller, target, "", true
}

// callerAndTarget is the resolveCallerAndTarget wrapper every handler that
// responds immediately (no deferred interaction) uses: it responds with an
// ephemeral error itself and returns ok=false on failure or self-targeting.
func callerAndTarget(s *discordgo.Session, i *discordgo.InteractionCreate) (caller, target int64, ok bool) {
	caller, target, failure, ok := resolveCallerAndTarget(s, i)
	if !ok {
		respondEphemeral(s, i, failure)
	}
	return caller, target, ok
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
			// and replies like /party_info and /relationships render raw
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
// after this must go through editDeferred or followupThenClearPlaceholder
// instead of respond/respondEphemeral/respondPublic, since the initial
// response slot is now spent.
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
	if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{Content: &message}); err != nil {
		logger.Error("edit deferred interaction response", "interaction", i.ID, "error", err)
	}
}

// followupThenClearPlaceholder posts the real response as a followup
// message carrying its own visibility (flags), then deletes the
// deferEphemeral placeholder so it doesn't linger as a stray "thinking..."
// bubble. Used by a handler whose outcome decides public vs ephemeral,
// where editDeferred can't apply since a followup message, unlike the
// deferred response, can set its own flags.
func followupThenClearPlaceholder(s *discordgo.Session, i *discordgo.InteractionCreate, message string, flags discordgo.MessageFlags) {
	if _, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{
		Content: message,
		Flags:   flags,
	}); err != nil {
		logger.Error("send followup interaction response", "interaction", i.ID, "error", err)
	}
	if err := s.InteractionResponseDelete(i.Interaction); err != nil {
		logger.Error("delete deferred interaction placeholder", "interaction", i.ID, "error", err)
	}
}
