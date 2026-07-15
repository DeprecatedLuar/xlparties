package commands

import (
	"fmt"
	"log"
	"strconv"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/party"
	"xlparties/internal/store"
)

const (
	overrideTypeAllow = "allow"
	overrideTypeDeny  = "deny"
)

func handleVCAllow(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store) {
	handleVCOverride(s, i, st, overrideTypeAllow)
}

func handleVCDeny(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store) {
	handleVCOverride(s, i, st, overrideTypeDeny)
}

func handleVCOverride(s *discordgo.Session, i *discordgo.InteractionCreate, st *store.Store, overrideType string) {
	caller, target, ok := callerAndTarget(s, i)
	if !ok {
		return
	}

	channelID, err := strconv.ParseInt(i.ChannelID, 10, 64)
	if err != nil {
		log.Printf("vc_%s: parse channel id: %v", overrideType, err)
		respondEphemeral(s, i, "failed to resolve the current channel")
		return
	}

	activeParty, found, err := st.PartyByChannel(channelID)
	if err != nil {
		log.Printf("vc_%s: lookup party: %v", overrideType, err)
		respondEphemeral(s, i, "failed to look up this party")
		return
	}
	if !found || activeParty.OwnerID != caller {
		respondEphemeral(s, i, "you must be the owner of the party channel you're in to use this command")
		return
	}

	var allow, deny int64
	if overrideType == overrideTypeAllow {
		allow = party.PartyChannelPermissions
	} else {
		deny = party.PartyChannelPermissions
	}
	targetID := strconv.FormatInt(target, 10)
	if err := s.ChannelPermissionSet(i.ChannelID, targetID, discordgo.PermissionOverwriteTypeMember, allow, deny); err != nil {
		log.Printf("vc_%s: set channel permission: %v", overrideType, err)
		respondEphemeral(s, i, fmt.Sprintf("failed to %s user", overrideType))
		return
	}
	if err := st.UpsertOverride(channelID, target, overrideType); err != nil {
		log.Printf("vc_%s: upsert override: %v", overrideType, err)
		respondEphemeral(s, i, fmt.Sprintf("failed to %s user", overrideType))
		return
	}

	if overrideType == overrideTypeAllow {
		respondPublic(s, i, fmt.Sprintf("<@%d> is now allowed in this party", target))
	} else {
		respondEphemeral(s, i, fmt.Sprintf("<@%d> is now denied from this party", target))
	}
}
