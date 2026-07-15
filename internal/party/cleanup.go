package party

import (
	"log"
	"strconv"
	"time"
)

// startCleanupTimer arms the empty-channel grace timer for channelID if one
// isn't already running.
func (m *Manager) startCleanupTimer(channelID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, running := m.cleanupTimers[channelID]; running {
		return
	}
	m.cleanupTimers[channelID] = time.AfterFunc(m.emptyCleanup, func() {
		m.runCleanup(channelID)
	})
}

// cancelCleanupTimer stops a pending cleanup timer for channelID, if any.
func (m *Manager) cancelCleanupTimer(channelID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if timer, running := m.cleanupTimers[channelID]; running {
		timer.Stop()
		delete(m.cleanupTimers, channelID)
	}
}

// runCleanup fires when a channel's empty-grace timer elapses. It
// re-checks membership before deleting, since the timer cannot be cancelled
// instantaneously from every code path.
func (m *Manager) runCleanup(channelID int64) {
	m.mu.Lock()
	delete(m.cleanupTimers, channelID)
	m.mu.Unlock()

	channelIDStr := strconv.FormatInt(channelID, 10)
	if len(m.membersInChannel(channelIDStr)) > 0 {
		return // someone joined; the join handler should have cancelled this timer, but guard anyway
	}

	m.deleteParty(channelID)
}

// deleteParty deletes the Discord channel (which clears its overwrites from
// the guild budget) and the corresponding parties/party_overrides rows.
func (m *Manager) deleteParty(channelID int64) {
	channelIDStr := strconv.FormatInt(channelID, 10)
	if _, err := m.session.ChannelDelete(channelIDStr); err != nil {
		log.Printf("cleanup: delete channel %d: %v", channelID, err)
		return
	}
	if err := m.store.DeleteOverridesForChannel(channelID); err != nil {
		log.Printf("cleanup: delete overrides for channel %d: %v", channelID, err)
	}
	if err := m.store.DeleteParty(channelID); err != nil {
		log.Printf("cleanup: delete party row for channel %d: %v", channelID, err)
		return
	}
	log.Printf("party cleaned up: channel=%d", channelID)
}

// StartupSweep reconciles the parties table against live Discord state after
// a restart, per spec.md Cleanup: rows whose channel no longer exists are
// removed, channels found empty get a fresh grace period, and non-empty
// channels resume with ownership re-evaluated against current membership.
func (m *Manager) StartupSweep() {
	parties, err := m.store.AllParties()
	if err != nil {
		log.Printf("startup sweep: load parties: %v", err)
		return
	}

	for _, p := range parties {
		exists, err := m.channelExists(p.ChannelID)
		if err != nil {
			log.Printf("startup sweep: check channel %d: %v", p.ChannelID, err)
			continue
		}
		if !exists {
			if err := m.store.DeleteOverridesForChannel(p.ChannelID); err != nil {
				log.Printf("startup sweep: delete overrides for orphan channel %d: %v", p.ChannelID, err)
			}
			if err := m.store.DeleteParty(p.ChannelID); err != nil {
				log.Printf("startup sweep: delete orphan party row %d: %v", p.ChannelID, err)
			}
			log.Printf("startup sweep: removed orphan party row for channel %d", p.ChannelID)
			continue
		}

		channelIDStr := strconv.FormatInt(p.ChannelID, 10)
		members := m.membersInChannel(channelIDStr)
		if len(members) == 0 {
			m.startCleanupTimer(p.ChannelID)
			continue
		}
		if !containsUser(members, p.OwnerID) {
			m.startHandoffTimer(p.ChannelID, p.OwnerID)
		}
	}

	log.Printf("startup sweep complete: %d parties checked", len(parties))
}
