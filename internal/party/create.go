package party

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/messages"
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
		logger.Info("party channel no longer exists on Discord, reclaiming slot", "channel", existing.ChannelID, "owner", ownerID)
	}

	mode, err := resolveAccessMode(m.store, ownerID)
	if err != nil {
		return fmt.Errorf("resolve access mode for owner %d: %w", ownerID, err)
	}

	var friendIDs []int64
	if mode != store.AccessModePublic {
		friendIDs, err = m.store.AllowedFriendIDs(ownerID)
		if err != nil {
			return fmt.Errorf("load friends for owner %d: %w", ownerID, err)
		}
	}

	categoryID, _, err := m.store.GetConfig(store.ConfigKeyCategory)
	if err != nil {
		return fmt.Errorf("load party category config: %w", err)
	}

	// Recorded before the Discord call so a crash between the channel
	// actually being created and InsertParty persisting it below leaves a
	// durable trace; ReconcileStaleCreations reads it back on next startup.
	if err := m.store.InsertPendingCreation(ownerID); err != nil {
		return fmt.Errorf("record pending creation for owner %d: %w", ownerID, err)
	}
	defer func() {
		if err := m.store.DeletePendingCreation(ownerID); err != nil {
			logger.Error("clear pending creation", "owner", ownerID, "error", err)
		}
	}()

	blockedIDs, err := m.store.BlockIDs(ownerID)
	if err != nil {
		return fmt.Errorf("load blocked users for owner %d: %w", ownerID, err)
	}

	overwrites, err := buildRewriteOverwrites(m.store, m.guildID, ownerID, mode, friendIDs, nil, nil, blockedIDs, nil)
	if err != nil {
		return fmt.Errorf("build overwrites for owner %d: %w", ownerID, err)
	}

	channel, err := m.session.GuildChannelCreateComplex(m.guildID, discordgo.GuildChannelCreateData{
		Name:                 naming.Generate(),
		Type:                 discordgo.ChannelTypeGuildVoice,
		ParentID:             categoryID, // empty string creates at guild root
		PermissionOverwrites: overwrites,
	})
	if err != nil {
		return fmt.Errorf("create party channel for owner %d: %w", ownerID, err)
	}

	channelID, err := strconv.ParseInt(channel.ID, 10, 64)
	if err != nil {
		return fmt.Errorf("parse created channel id %q: %w", channel.ID, err)
	}
	if err := m.store.InsertParty(channelID, ownerID, mode); err != nil {
		if _, delErr := m.session.ChannelDelete(channel.ID); delErr != nil {
			logger.Error("rollback: delete channel after failed party insert", "channel", channelID, "error", delErr)
		}
		return fmt.Errorf("insert party row for channel %d: %w", channelID, err)
	}

	ownerIDStr := strconv.FormatInt(ownerID, 10)
	if err := m.session.GuildMemberMove(m.guildID, ownerIDStr, &channel.ID); err != nil {
		return fmt.Errorf("move owner %d into party channel %d: %w", ownerID, channelID, err)
	}

	// Send salutations message to the new channel's text chat
	salutation := fmt.Sprintf(messages.PartyCreated, ownerID, len(friendIDs))
	if mode == store.AccessModePublic {
		salutation = fmt.Sprintf(messages.PartyCreatedPublic, ownerID)
	}
	if _, err := m.session.ChannelMessageSend(channel.ID, salutation); err != nil {
		logger.Error("party creation: post salutations", "channel", channelID, "error", err)
	}
	if mode != store.AccessModePublic && len(friendIDs) == 0 {
		if _, err := m.session.ChannelMessageSend(channel.ID, messages.PartyCreatedNoFriendsWarning); err != nil {
			logger.Error("party creation: post no-friends warning", "channel", channelID, "error", err)
		}
	}

	logger.Info("party created", "channel", channelID, "owner", ownerID, "friends", len(friendIDs))
	return nil
}

// resolveAccessMode returns ownerID's saved preset mode, falling back to
// store.DefaultAccessMode when no preset is set. A preset only ever affects
// the party being created here - it never rewrites an existing one.
func resolveAccessMode(st *store.Store, ownerID int64) (string, error) {
	mode, found, err := st.PresetForUser(ownerID)
	if err != nil {
		return "", fmt.Errorf("load preset for owner %d: %w", ownerID, err)
	}
	if !found {
		return store.DefaultAccessMode, nil
	}
	return mode, nil
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
