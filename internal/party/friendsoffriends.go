package party

import (
	"slices"
	"strconv"
	"time"

	"xlparties/internal/logger"
	"xlparties/internal/store"
)

// friendOfFriendJoinDelay is how long a non-owner member must stay present in
// a friends_of_friends party before they become an active scan source (their
// friends gain access).
const friendOfFriendJoinDelay = 5 * time.Second

// friendOfFriendLeaveGrace is how long an active source's contribution
// survives after they leave, before it drops out of the crawl.
const friendOfFriendLeaveGrace = 15 * time.Second

// startFoFJoinTimer arms the maturation timer for (channelID, userID) if one
// isn't already running.
func (m *Manager) startFoFJoinTimer(channelID, userID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.fofJoinTimers[channelID]; !ok {
		m.fofJoinTimers[channelID] = make(map[int64]*time.Timer)
	}
	if _, running := m.fofJoinTimers[channelID][userID]; running {
		return
	}
	m.fofJoinTimers[channelID][userID] = time.AfterFunc(friendOfFriendJoinDelay, func() {
		m.runFoFJoinScan(channelID, userID)
	})
}

// cancelFoFJoinTimer stops a pending maturation timer for (channelID,
// userID), if any.
func (m *Manager) cancelFoFJoinTimer(channelID, userID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	timers, ok := m.fofJoinTimers[channelID]
	if !ok {
		return
	}
	if timer, running := timers[userID]; running {
		timer.Stop()
		delete(timers, userID)
	}
}

// startFoFLeaveTimer arms the leave-grace timer for (channelID, userID) if
// one isn't already running.
func (m *Manager) startFoFLeaveTimer(channelID, userID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.fofLeaveTimers[channelID]; !ok {
		m.fofLeaveTimers[channelID] = make(map[int64]*time.Timer)
	}
	if _, running := m.fofLeaveTimers[channelID][userID]; running {
		return
	}
	m.fofLeaveTimers[channelID][userID] = time.AfterFunc(friendOfFriendLeaveGrace, func() {
		m.runFoFLeaveRevoke(channelID, userID)
	})
}

// cancelFoFLeaveTimer stops a pending leave-grace timer for (channelID,
// userID), if any.
func (m *Manager) cancelFoFLeaveTimer(channelID, userID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	timers, ok := m.fofLeaveTimers[channelID]
	if !ok {
		return
	}
	if timer, running := timers[userID]; running {
		timer.Stop()
		delete(timers, userID)
	}
}

// cancelFoFTimersForChannel stops and drops every pending join/leave timer
// for channelID. Shared by cleanup (channel deleted) and the visibility
// toggle (switch to friends-only).
func (m *Manager) cancelFoFTimersForChannel(channelID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, timer := range m.fofJoinTimers[channelID] {
		timer.Stop()
	}
	delete(m.fofJoinTimers, channelID)
	for _, timer := range m.fofLeaveTimers[channelID] {
		timer.Stop()
	}
	delete(m.fofLeaveTimers, channelID)
}

// runFoFJoinScan fires when a member's join-delay timer elapses. It
// re-validates against current state before acting - the member may have
// left again (cancelFoFJoinTimer should have caught that, but the timer
// can't be cancelled instantaneously from every path), the party may have
// switched to friends-only, or been deleted.
func (m *Manager) runFoFJoinScan(channelID, userID int64) {
	m.mu.Lock()
	if timers, ok := m.fofJoinTimers[channelID]; ok {
		delete(timers, userID)
	}
	m.mu.Unlock()

	party, exists, err := m.store.PartyByChannel(channelID)
	if err != nil {
		logger.Error("fof join scan: load party", "channel", channelID, "error", err)
		return
	}
	if !exists || party.AccessMode != store.AccessModeFriendsOfFriends || party.OwnerID == userID {
		return
	}
	if !containsUser(m.membersInChannel(strconv.FormatInt(channelID, 10)), userID) {
		return // left before maturing
	}

	if err := m.store.AddSource(channelID, userID); err != nil {
		logger.Error("fof join scan: add source", "channel", channelID, "user", userID, "error", err)
		return
	}
	if err := m.rewriteOverwrites(channelID, party.OwnerID); err != nil {
		logger.Error("fof join scan: rewrite overwrites", "channel", channelID, "error", err)
		return
	}
	logger.Info("fof join scan: added source", "channel", channelID, "user", userID)
}

// runFoFLeaveRevoke fires when a source's leave-grace timer elapses. It
// re-validates against current state before acting: the member may have
// rejoined (cancelFoFLeaveTimer should have caught that, guarded anyway), or
// the party may have switched to friends-only or been deleted. Note this
// naturally preserves access granted by another still-present source: the
// allow-set is a live union of every source's friends, so removing one
// source's row doesn't revoke access another source still vouches for.
func (m *Manager) runFoFLeaveRevoke(channelID, userID int64) {
	m.mu.Lock()
	if timers, ok := m.fofLeaveTimers[channelID]; ok {
		delete(timers, userID)
	}
	m.mu.Unlock()

	party, exists, err := m.store.PartyByChannel(channelID)
	if err != nil {
		logger.Error("fof leave revoke: load party", "channel", channelID, "error", err)
		return
	}
	if !exists || party.AccessMode != store.AccessModeFriendsOfFriends {
		return
	}
	if containsUser(m.membersInChannel(strconv.FormatInt(channelID, 10)), userID) {
		return // rejoined before grace elapsed
	}

	if err := m.store.RemoveSource(channelID, userID); err != nil {
		logger.Error("fof leave revoke: remove source", "channel", channelID, "user", userID, "error", err)
		return
	}
	if err := m.rewriteOverwrites(channelID, party.OwnerID); err != nil {
		logger.Error("fof leave revoke: rewrite overwrites", "channel", channelID, "error", err)
		return
	}
	logger.Info("fof leave revoke: removed source", "channel", channelID, "user", userID)
}

// reconcileFoFSources resyncs a friends_of_friends party's active scan
// sources against current channel membership after a restart, since the
// join/leave timers driving party_sources are in-memory and don't survive
// one. Sources who left while the bot was down are pruned - their leave
// timer never got a chance to fire, so without this they'd keep granting
// access indefinitely. Members present but not yet a source get a fresh
// join-maturation timer; any partial progress from before the restart is
// lost, the same accepted cost already applied to the handoff/cleanup
// timers. members is the channel's current non-bot occupants, already
// computed by the caller (StartupSweep).
func (m *Manager) reconcileFoFSources(p store.Party, members []string) {
	if p.AccessMode != store.AccessModeFriendsOfFriends {
		return
	}

	sourceIDs, err := m.store.SourceIDsForChannel(p.ChannelID)
	if err != nil {
		logger.Error("reconcile fof sources: load sources for channel", "channel", p.ChannelID, "error", err)
		return
	}

	pruned := false
	for _, sourceID := range sourceIDs {
		if containsUser(members, sourceID) {
			continue
		}
		if err := m.store.RemoveSource(p.ChannelID, sourceID); err != nil {
			logger.Error("reconcile fof sources: remove stale source", "channel", p.ChannelID, "user", sourceID, "error", err)
			continue
		}
		pruned = true
		logger.Info("reconcile fof sources: pruned stale source", "channel", p.ChannelID, "user", sourceID)
	}
	if pruned {
		if err := m.rewriteOverwrites(p.ChannelID, p.OwnerID); err != nil {
			logger.Error("reconcile fof sources: rewrite overwrites", "channel", p.ChannelID, "error", err)
		}
	}

	for _, member := range members {
		memberID, err := strconv.ParseInt(member, 10, 64)
		if err != nil {
			logger.Error("reconcile fof sources: parse member id", "member_id", member, "error", err)
			continue
		}
		if memberID == p.OwnerID || slices.Contains(sourceIDs, memberID) {
			continue
		}
		m.startFoFJoinTimer(p.ChannelID, memberID)
	}
}
