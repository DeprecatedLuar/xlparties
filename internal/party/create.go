package party

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/messages"
	"xlparties/internal/naming"
	"xlparties/internal/store"
)

// unjoinedCleanupGrace is the empty-channel grace period applied to a party
// created via CreateParty when the owner wasn't moved in (they weren't
// connected to voice at creation time). m.emptyCleanup is tuned for
// "everyone just left a live channel" and is too tight for "create it, then
// go join it".
const unjoinedCleanupGrace = 2 * time.Minute

// spawnParty is the voice-state-triggered entry point: it always tries to
// move ownerID into the resulting channel, using their saved preset for
// mode and limit.
func (m *Manager) spawnParty(ownerID int64) error {
	_, _, err := m.CreateParty(ownerID, "", nil)
	return err
}

// CreateParty moves ownerID into their existing party channel if it still
// exists on Discord, reclaims the owner's slot if that channel was deleted
// out-of-band (e.g. manually in Discord), or creates a fresh party channel
// otherwise. modeOverride and limitOverride, when non-empty/non-nil, win
// over the owner's saved /party_preset (used by /party_create); pass ""
// and nil to fall back to the preset the way joining the watch channel
// does. The owner is moved into the channel only if they are already
// connected to voice somewhere in the guild - otherwise CreateParty leaves
// them to join on their own and arms unjoinedCleanupGrace instead of
// m.emptyCleanup so an unused channel doesn't linger.
func (m *Manager) CreateParty(ownerID int64, modeOverride string, limitOverride *int) (channelID int64, alreadyExisted bool, err error) {
	existing, exists, err := m.store.PartyByOwner(ownerID)
	if err != nil {
		return 0, false, fmt.Errorf("check existing party for owner %d: %w", ownerID, err)
	}
	if exists {
		channelStillExists, err := m.channelExists(existing.ChannelID)
		if err != nil {
			return 0, false, fmt.Errorf("check party channel %d exists: %w", existing.ChannelID, err)
		}
		if channelStillExists {
			if err := m.moveOwnerIfConnected(ownerID, existing.ChannelID); err != nil {
				return 0, false, err
			}
			return existing.ChannelID, true, nil
		}
		// Channel was deleted out-of-band; reclaim the owner's slot and fall
		// through to create a fresh party.
		if err := m.store.DeleteParty(existing.ChannelID); err != nil {
			return 0, false, fmt.Errorf("delete stale party row for channel %d: %w", existing.ChannelID, err)
		}
		logger.Info("party channel no longer exists on Discord, reclaiming slot", "channel", existing.ChannelID, "owner", ownerID)
	}

	mode, hasPreset, err := resolveAccessMode(m.store, ownerID, modeOverride)
	if err != nil {
		return 0, false, fmt.Errorf("resolve access mode for owner %d: %w", ownerID, err)
	}

	userLimit, err := resolveUserLimit(m.store, ownerID, limitOverride)
	if err != nil {
		return 0, false, fmt.Errorf("resolve user limit for owner %d: %w", ownerID, err)
	}

	var friendIDs []int64
	if mode != store.AccessModePublic {
		friendIDs, err = m.store.AllowedFriendIDs(ownerID)
		if err != nil {
			return 0, false, fmt.Errorf("load friends for owner %d: %w", ownerID, err)
		}
	}

	categoryID, _, err := m.store.GetConfig(store.ConfigKeyCategory)
	if err != nil {
		return 0, false, fmt.Errorf("load party category config: %w", err)
	}

	// Recorded before the Discord call so a crash between the channel
	// actually being created and InsertParty persisting it below leaves a
	// durable trace; ReconcileStaleCreations reads it back on next startup.
	if err := m.store.InsertPendingCreation(ownerID); err != nil {
		return 0, false, fmt.Errorf("record pending creation for owner %d: %w", ownerID, err)
	}
	defer func() {
		if err := m.store.DeletePendingCreation(ownerID); err != nil {
			logger.Error("clear pending creation", "owner", ownerID, "error", err)
		}
	}()

	blockedIDs, err := m.store.BlockIDs(ownerID)
	if err != nil {
		return 0, false, fmt.Errorf("load blocked users for owner %d: %w", ownerID, err)
	}

	overwrites, err := buildRewriteOverwrites(m.store, m.guildID, m.botID, ownerID, mode, friendIDs, nil, nil, blockedIDs, nil)
	if err != nil {
		return 0, false, fmt.Errorf("build overwrites for owner %d: %w", ownerID, err)
	}

	channel, err := m.session.GuildChannelCreateComplex(m.guildID, discordgo.GuildChannelCreateData{
		Name:                 naming.Generate(),
		Type:                 discordgo.ChannelTypeGuildVoice,
		ParentID:             categoryID, // empty string creates at guild root
		PermissionOverwrites: overwrites,
		UserLimit:            userLimit,
	})
	if err != nil {
		return 0, false, fmt.Errorf("create party channel for owner %d: %w", ownerID, err)
	}

	newChannelID, err := strconv.ParseInt(channel.ID, 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("parse created channel id %q: %w", channel.ID, err)
	}
	if err := m.store.InsertParty(newChannelID, ownerID, mode); err != nil {
		if _, delErr := m.session.ChannelDelete(channel.ID); delErr != nil {
			logger.Error("rollback: delete channel after failed party insert", "channel", newChannelID, "error", delErr)
		}
		return 0, false, fmt.Errorf("insert party row for channel %d: %w", newChannelID, err)
	}

	moved, err := m.moveOwnerIfConnectedTo(ownerID, channel.ID)
	if err != nil {
		return 0, false, fmt.Errorf("move owner %d into party channel %d: %w", ownerID, newChannelID, err)
	}
	if !moved {
		m.startCleanupTimerAfter(newChannelID, unjoinedCleanupGrace)
	}

	// Send salutations message to the new channel's text chat
	salutation := fmt.Sprintf(messages.PartyCreated, ownerID, messages.AccessModeLabel[mode], len(friendIDs))
	if mode == store.AccessModePublic {
		salutation = fmt.Sprintf(messages.PartyCreatedPublic, ownerID)
	}
	if !hasPreset {
		salutation += "\n\n" + messages.PartyPresetTip
	}
	if _, err := m.session.ChannelMessageSend(channel.ID, salutation); err != nil {
		logger.Error("party creation: post salutations", "channel", newChannelID, "error", err)
	}
	if mode != store.AccessModePublic && len(friendIDs) == 0 {
		if _, err := m.session.ChannelMessageSend(channel.ID, messages.PartyCreatedNoFriendsWarning); err != nil {
			logger.Error("party creation: post no-friends warning", "channel", newChannelID, "error", err)
		}
	}

	logger.Info("party created", "channel", newChannelID, "owner", ownerID, "friends", len(friendIDs), "moved", moved)
	return newChannelID, false, nil
}

// resolveAccessMode returns the mode to create the party in and whether it
// should be treated as coming from a saved preset (for messaging purposes):
// modeOverride wins outright when non-empty (validated against the known
// store.AccessMode* values), otherwise ownerID's saved preset is used,
// falling back to store.DefaultAccessMode when neither is set. A preset (or
// override) only ever affects the party being created here, never rewrites
// an existing one.
func resolveAccessMode(st *store.Store, ownerID int64, modeOverride string) (string, bool, error) {
	if modeOverride != "" {
		switch modeOverride {
		case store.AccessModeFriendsOfFriends, store.AccessModeFriendsOnly, store.AccessModeInviteOnly, store.AccessModePublic:
			return modeOverride, true, nil
		default:
			return "", false, fmt.Errorf("unknown access mode %q", modeOverride)
		}
	}

	mode, found, err := st.PresetForUser(ownerID)
	if err != nil {
		return "", false, fmt.Errorf("load preset for owner %d: %w", ownerID, err)
	}
	if !found {
		return store.DefaultAccessMode, false, nil
	}
	return mode, true, nil
}

// resolveUserLimit returns the user limit to create the party with:
// limitOverride wins outright when non-nil (validated against Discord's own
// voice channel bounds), otherwise falls back to ownerID's saved preset
// limit, or 0 (unlimited) when neither is set.
func resolveUserLimit(st *store.Store, ownerID int64, limitOverride *int) (int, error) {
	if limitOverride != nil {
		if *limitOverride < minUserLimit || *limitOverride > maxUserLimit {
			return 0, fmt.Errorf("user limit %d out of range [%d, %d]", *limitOverride, minUserLimit, maxUserLimit)
		}
		return *limitOverride, nil
	}

	limit, found, err := st.PresetLimitForUser(ownerID)
	if err != nil {
		return 0, fmt.Errorf("load preset limit for owner %d: %w", ownerID, err)
	}
	if !found {
		return 0, nil
	}
	return limit, nil
}

// moveOwnerIfConnectedTo moves ownerID into channelIDStr if they are
// currently connected to any voice channel in the guild, reporting whether
// the move happened. Not being connected is not an error - CreateParty
// calls this so /party_create can create a party without the caller
// already being in voice.
func (m *Manager) moveOwnerIfConnectedTo(ownerID int64, channelIDStr string) (bool, error) {
	ownerIDStr := strconv.FormatInt(ownerID, 10)
	if !m.memberConnected(ownerIDStr) {
		return false, nil
	}
	if err := m.session.GuildMemberMove(m.guildID, ownerIDStr, &channelIDStr); err != nil {
		return false, err
	}
	return true, nil
}

// moveOwnerIfConnected is the existing-party variant of
// moveOwnerIfConnectedTo, taking channelID as an int64 the way the rest of
// the existing-party branch does.
func (m *Manager) moveOwnerIfConnected(ownerID, channelID int64) error {
	channelIDStr := strconv.FormatInt(channelID, 10)
	_, err := m.moveOwnerIfConnectedTo(ownerID, channelIDStr)
	if err != nil {
		return fmt.Errorf("move owner %d into existing party channel %d: %w", ownerID, channelID, err)
	}
	return nil
}

// memberConnected reports whether userIDStr currently has a voice state in
// the guild (i.e. is connected to some voice channel).
func (m *Manager) memberConnected(userIDStr string) bool {
	guild, err := m.session.State.Guild(m.guildID)
	if err != nil {
		logger.Error("check member voice state", "error", err)
		return false
	}
	for _, vs := range guild.VoiceStates {
		if vs.UserID == userIDStr {
			return true
		}
	}
	return false
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
	if errors.As(err, &restErr) && restErr.Message != nil && restErr.Message.Code == discordgo.ErrCodeMissingAccess {
		return false, fmt.Errorf("bot is locked out of party channel %d (missing access) — re-grant the bot access or delete the channel manually", channelID)
	}
	return false, err
}
