package party

import (
	"fmt"
	"log"
	"strconv"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/naming"
	"xlparties/internal/store"
)

// spawnParty creates a private party channel for ownerID and moves them into
// it, unless they already own an active party.
func (m *Manager) spawnParty(ownerID int64) error {
	_, exists, err := m.store.PartyByOwner(ownerID)
	if err != nil {
		return fmt.Errorf("check existing party for owner %d: %w", ownerID, err)
	}
	if exists {
		return nil // duplicate guard: no second party per owner
	}

	friendIDs, err := m.store.FriendIDs(ownerID)
	if err != nil {
		return fmt.Errorf("load friends for owner %d: %w", ownerID, err)
	}

	categoryID, _, err := m.store.GetConfig(store.ConfigKeyCategory)
	if err != nil {
		return fmt.Errorf("load party category config: %w", err)
	}

	channel, err := m.session.GuildChannelCreateComplex(m.guildID, discordgo.GuildChannelCreateData{
		Name:                 naming.Generate(),
		Type:                 discordgo.ChannelTypeGuildVoice,
		ParentID:             categoryID, // empty string creates at guild root
		PermissionOverwrites: buildCreationOverwrites(m.guildID, ownerID, friendIDs),
	})
	if err != nil {
		return fmt.Errorf("create party channel for owner %d: %w", ownerID, err)
	}

	channelID, err := strconv.ParseInt(channel.ID, 10, 64)
	if err != nil {
		return fmt.Errorf("parse created channel id %q: %w", channel.ID, err)
	}
	if err := m.store.InsertParty(channelID, ownerID); err != nil {
		return fmt.Errorf("insert party row for channel %d: %w", channelID, err)
	}

	ownerIDStr := strconv.FormatInt(ownerID, 10)
	if err := m.session.GuildMemberMove(m.guildID, ownerIDStr, &channel.ID); err != nil {
		return fmt.Errorf("move owner %d into party channel %d: %w", ownerID, channelID, err)
	}

	log.Printf("party created: channel=%d owner=%d friends=%d", channelID, ownerID, len(friendIDs))
	return nil
}
