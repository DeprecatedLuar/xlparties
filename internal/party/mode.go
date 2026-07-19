package party

import (
	"fmt"
	"strconv"

	"xlparties/internal/logger"
	"xlparties/internal/store"
)

// SetAccessMode switches channelID's access mode, per spec.md Per-Channel
// Access Modes, persists it, and rewrites the channel's overwrites to match.
// Leaving friends_of_friends drops all active scan sources and cancels
// their timers, since neither applies outside that mode. Entering
// friends_of_friends arms a fresh join-maturation timer for every non-owner
// member currently present, so growth resumes without requiring a
// leave/rejoin.
func (m *Manager) SetAccessMode(channelID int64, mode string) error {
	switch mode {
	case store.AccessModeFriendsOfFriends, store.AccessModeFriendsOnly, store.AccessModeInviteOnly, store.AccessModePublic:
	default:
		return fmt.Errorf("unknown access mode %q", mode)
	}

	p, exists, err := m.store.PartyByChannel(channelID)
	if err != nil {
		return fmt.Errorf("load party %d: %w", channelID, err)
	}
	if !exists {
		return fmt.Errorf("party %d not found", channelID)
	}
	previousMode := p.AccessMode

	if err := m.store.UpdateAccessMode(channelID, mode); err != nil {
		return err
	}

	if previousMode == store.AccessModeFriendsOfFriends && mode != store.AccessModeFriendsOfFriends {
		m.cancelFoFTimersForChannel(channelID)
		if err := m.store.RemoveSourcesForChannel(channelID); err != nil {
			return fmt.Errorf("remove sources for channel %d: %w", channelID, err)
		}
	}

	if err := m.rewriteOverwrites(channelID, p.OwnerID); err != nil {
		return fmt.Errorf("rewrite overwrites for channel %d: %w", channelID, err)
	}

	if previousMode != store.AccessModeFriendsOfFriends && mode == store.AccessModeFriendsOfFriends {
		channelIDStr := strconv.FormatInt(channelID, 10)
		for _, member := range m.membersInChannel(channelIDStr) {
			memberID, err := strconv.ParseInt(member, 10, 64)
			if err != nil {
				logger.Error("set access mode: parse member id", "member_id", member, "error", err)
				continue
			}
			if memberID == p.OwnerID {
				continue
			}
			m.startFoFJoinTimer(channelID, memberID)
		}
	}

	logger.Info("party mode changed", "channel", channelID, "from", previousMode, "to", mode)
	return nil
}
