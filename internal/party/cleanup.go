package party

import (
	"log"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/store"
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

// SweepOrphanChannels deletes voice channels sitting in the configured party
// category that have no corresponding row in the parties table. This is the
// complement to StartupSweep: StartupSweep walks known DB rows out to
// Discord, this walks live Discord channels back in against the DB, which is
// the only way to catch a channel whose creation crashed between the Discord
// create call and the DB insert (see internal/party/create.go spawnParty).
func (m *Manager) SweepOrphanChannels() {
	categoryID, ok, err := m.store.GetConfig(store.ConfigKeyCategory)
	if err != nil {
		log.Printf("self heal: load party category config: %v", err)
		return
	}
	if !ok {
		log.Println("self heal: party_category not configured, skipping orphan channel sweep")
		return
	}

	channels, err := m.session.GuildChannels(m.guildID)
	if err != nil {
		log.Printf("self heal: list guild channels: %v", err)
		return
	}

	parties, err := m.store.AllParties()
	if err != nil {
		log.Printf("self heal: load parties: %v", err)
		return
	}
	known := make(map[int64]struct{}, len(parties))
	for _, p := range parties {
		known[p.ChannelID] = struct{}{}
	}

	swept := 0
	for _, c := range channels {
		if c.Type != discordgo.ChannelTypeGuildVoice || c.ParentID != categoryID {
			continue
		}
		channelID, err := strconv.ParseInt(c.ID, 10, 64)
		if err != nil {
			log.Printf("self heal: parse channel id %q: %v", c.ID, err)
			continue
		}
		if _, tracked := known[channelID]; tracked {
			continue
		}
		if _, err := m.session.ChannelDelete(c.ID); err != nil {
			log.Printf("self heal: delete orphan channel %d: %v", channelID, err)
			continue
		}
		swept++
		log.Printf("self heal: deleted orphan channel %d (no parties record)", channelID)
	}

	log.Printf("self heal: orphan channel sweep complete: %d channels removed", swept)
}
