package party

import (
	"strconv"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
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
		logger.Error("cleanup: delete channel", "channel", channelID, "error", err)
		return
	}
	m.cancelFoFTimersForChannel(channelID)
	if err := m.store.RemoveSourcesForChannel(channelID); err != nil {
		logger.Error("cleanup: remove sources for channel", "channel", channelID, "error", err)
	}
	m.cancelInviteTimersForChannel(channelID)
	if err := m.store.RemoveInvitesForChannel(channelID); err != nil {
		logger.Error("cleanup: remove invites for channel", "channel", channelID, "error", err)
	}
	if err := m.store.DeleteOverridesForChannel(channelID); err != nil {
		logger.Error("cleanup: delete overrides for channel", "channel", channelID, "error", err)
	}
	if err := m.store.DeleteParty(channelID); err != nil {
		logger.Error("cleanup: delete party row for channel", "channel", channelID, "error", err)
		return
	}
	logger.Info("party cleaned up", "channel", channelID)
}

// sweepConcurrency bounds how many channelExists lookups StartupSweep issues
// to Discord's REST API at once, so a large parties table doesn't serialize
// one HTTP round-trip per row on every restart.
const sweepConcurrency = 8

// sweepResult pairs a party row with the outcome of its channelExists check,
// so StartupSweep can run the REST lookups concurrently while still applying
// their results (store writes, timer starts) one at a time.
type sweepResult struct {
	party  store.Party
	exists bool
	err    error
}

// StartupSweep reconciles the parties table against live Discord state after
// a restart, per spec.md Cleanup: rows whose channel no longer exists are
// removed, channels found empty get a fresh grace period, and non-empty
// channels resume with ownership re-evaluated against current membership.
func (m *Manager) StartupSweep() {
	parties, err := m.store.AllParties()
	if err != nil {
		logger.Error("startup sweep: load parties", "error", err)
		return
	}

	results := make([]sweepResult, len(parties))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for range sweepConcurrency {
		wg.Go(func() {
			for i := range jobs {
				exists, err := m.channelExists(parties[i].ChannelID)
				results[i] = sweepResult{party: parties[i], exists: exists, err: err}
			}
		})
	}
	for i := range parties {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	for _, res := range results {
		p := res.party
		if res.err != nil {
			logger.Error("startup sweep: check channel", "channel", p.ChannelID, "error", res.err)
			continue
		}
		if !res.exists {
			if err := m.store.DeleteOverridesForChannel(p.ChannelID); err != nil {
				logger.Error("startup sweep: delete overrides for orphan channel", "channel", p.ChannelID, "error", err)
			}
			if err := m.store.DeleteParty(p.ChannelID); err != nil {
				logger.Error("startup sweep: delete orphan party row", "channel", p.ChannelID, "error", err)
			}
			logger.Info("startup sweep: removed orphan party row", "channel", p.ChannelID)
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
		m.reconcileFoFSources(p, members)
	}

	m.reconcileInvites()

	logger.Info("startup sweep complete", "parties_checked", len(parties))
}

// reconcileInvites resyncs pending party_invites against live Discord state
// after a restart, since the expiry timers driving them are in-memory and
// don't survive one: a channel that's gone gets its row dropped, an invite
// already past its TTL is revoked immediately, and a still-live invite gets
// a fresh timer armed for its remaining TTL.
func (m *Manager) reconcileInvites() {
	invites, err := m.store.AllInvites()
	if err != nil {
		logger.Error("startup sweep: load invites", "error", err)
		return
	}

	now := time.Now().Unix()
	for _, inv := range invites {
		exists, err := m.channelExists(inv.ChannelID)
		if err != nil {
			logger.Error("startup sweep: check invite channel", "channel", inv.ChannelID, "user", inv.UserID, "error", err)
			continue
		}
		if !exists {
			if err := m.store.RemoveInvite(inv.ChannelID, inv.UserID); err != nil {
				logger.Error("startup sweep: remove invite for orphan channel", "channel", inv.ChannelID, "user", inv.UserID, "error", err)
			}
			continue
		}
		if inv.ExpiresAt <= now {
			m.revokeInvite(inv.ChannelID, inv.UserID)
			continue
		}
		m.startInviteExpiryTimer(inv.ChannelID, inv.UserID, time.Duration(inv.ExpiresAt-now)*time.Second)
	}
}

// SweepOrphanChannels reports (but does not delete) voice channels sitting
// in the configured party category that have no corresponding row in the
// parties table.
//
// DRY-RUN ONLY. It does not delete anything. The party category is not
// guaranteed to hold only bot-created channels — on at least one deployment
// it's the server's general voice category, shared with manually-created
// permanent channels — so "untracked channel in this category" is not a
// safe-to-delete signal on its own. This incorrectly deleted two real,
// manually-created channels in production before being changed to dry-run;
// do not re-enable deletion here without a reliable way to distinguish
// bot-created channels (e.g. tagging on create) from pre-existing ones.
func (m *Manager) SweepOrphanChannels() {
	categoryID, ok, err := m.store.GetConfig(store.ConfigKeyCategory)
	if err != nil {
		logger.Error("self heal: load party category config", "error", err)
		return
	}
	if !ok {
		logger.Warn("self heal: party_category not configured, skipping orphan channel sweep")
		return
	}

	channels, err := m.session.GuildChannels(m.guildID)
	if err != nil {
		logger.Error("self heal: list guild channels", "error", err)
		return
	}

	parties, err := m.store.AllParties()
	if err != nil {
		logger.Error("self heal: load parties", "error", err)
		return
	}
	known := make(map[int64]struct{}, len(parties))
	for _, p := range parties {
		known[p.ChannelID] = struct{}{}
	}

	found := 0
	for _, c := range channels {
		if c.Type != discordgo.ChannelTypeGuildVoice || c.ParentID != categoryID {
			continue
		}
		channelID, err := strconv.ParseInt(c.ID, 10, 64)
		if err != nil {
			logger.Error("self heal: parse channel id", "channel_id", c.ID, "error", err)
			continue
		}
		if _, tracked := known[channelID]; tracked {
			continue
		}
		found++
		logger.Warn("self heal: untracked channel in party category (not deleted, dry-run only, verify manually)", "channel", channelID, "name", c.Name)
	}

	logger.Info("self heal: orphan channel sweep complete (dry-run, none deleted)", "untracked_found", found)
}
