package commands

import (
	"fmt"
	"log"
	"strconv"

	"github.com/bwmarrin/discordgo"
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

// callerAndTarget resolves the caller and the "user" option, responding with
// an ephemeral error and returning ok=false on failure or self-targeting.
func callerAndTarget(s *discordgo.Session, i *discordgo.InteractionCreate) (caller, target int64, ok bool) {
	caller, err := callerID(i)
	if err != nil {
		log.Printf("resolve caller id: %v", err)
		respondEphemeral(s, i, "failed to resolve your user id")
		return 0, 0, false
	}
	target, err = userOptionID(i)
	if err != nil {
		log.Printf("resolve target user id: %v", err)
		respondEphemeral(s, i, "failed to resolve target user id")
		return 0, 0, false
	}
	if target == caller {
		respondEphemeral(s, i, "you cannot target yourself")
		return 0, 0, false
	}
	return caller, target, true
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
		},
	})
	if err != nil {
		fmt.Printf("failed to respond to interaction %q: %v\n", i.ID, err)
	}
}
