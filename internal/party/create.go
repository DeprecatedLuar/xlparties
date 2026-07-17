package party

import (
	"errors"
	"fmt"
	"log"
	"strconv"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/naming"
	"xlparties/internal/store"
)

// spawnParty moves ownerID into their existing party channel if it still
// exists on Discord, reclaims the owner's slot if that channel was deleted
// out-of-band (e.g. manually in Discord), or creates a fresh party channel
// otherwise.
func (m *Manager) spawnParty(ownerID int64) error {
	existing, exists, err := m.store.PartyByOwner(ownerID)
	if err != nil {
		return fmt.Errorf("check existing party for owner %d: %w", ownerID, err)
	}
	if exists {
		channelStillExists, err := m.channelExists(existing.ChannelID)
		if err != nil {
			return fmt.Errorf("check party channel %d exists: %w", existing.ChannelID, err)
		}
		if channelStillExists {
			ownerIDStr := strconv.FormatInt(ownerID, 10)
			channelIDStr := strconv.FormatInt(existing.ChannelID, 10)
			if err := m.session.GuildMemberMove(m.guildID, ownerIDStr, &channelIDStr); err != nil {
				return fmt.Errorf("move owner %d into existing party channel %d: %w", ownerID, existing.ChannelID, err)
			}
			return nil
		}
		// Channel was deleted out-of-band; reclaim the owner's slot and fall
		// through to create a fresh party.
		if err := m.store.DeleteParty(existing.ChannelID); err != nil {
			return fmt.Errorf("delete stale party row for channel %d: %w", existing.ChannelID, err)
		}
		log.Printf("party channel %d for owner %d no longer exists on Discord, reclaiming slot", existing.ChannelID, ownerID)
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
		if _, delErr := m.session.ChannelDelete(channel.ID); delErr != nil {
			log.Printf("rollback: delete channel %d after failed party insert: %v", channelID, delErr)
		}
		return fmt.Errorf("insert party row for channel %d: %w", channelID, err)
	}

	ownerIDStr := strconv.FormatInt(ownerID, 10)
	if err := m.session.GuildMemberMove(m.guildID, ownerIDStr, &channel.ID); err != nil {
		return fmt.Errorf("move owner %d into party channel %d: %w", ownerID, channelID, err)
	}

	log.Printf("party created: channel=%d owner=%d friends=%d", channelID, ownerID, len(friendIDs))
	return nil
}

// channelExists reports whether channelID still exists on Discord,
// distinguishing an expected "unknown channel" response (the channel was
// deleted out-of-band) from a real API failure.
func (m *Manager) channelExists(channelID int64) (bool, error) {
	channelIDStr := strconv.FormatInt(channelID, 10)
	_, err := m.session.Channel(channelIDStr)
	if err == nil {
		return true, nil
	}
	var restErr *discordgo.RESTError
	if errors.As(err, &restErr) && restErr.Message != nil && restErr.Message.Code == discordgo.ErrCodeUnknownChannel {
		return false, nil
	}
	return false, err
}
