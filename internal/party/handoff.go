package party

import (
	"fmt"
	"math/rand"
	"slices"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/messages"
	"xlparties/internal/store"
)

// startHandoffTimer arms the owner-absence handoff timer for channelID if
// one isn't already running. ownerID is the owner whose absence triggered
// this timer, captured so runHandoff can detect a stale fire (e.g. a
// handoff already happened, or the owner returned and the timer should have
// been cancelled).
func (m *Manager) startHandoffTimer(channelID, ownerID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, running := m.handoffTimers[channelID]; running {
		return
	}
	m.handoffTimers[channelID] = time.AfterFunc(m.ownerAbsenceHandoff, func() {
		m.runHandoff(channelID, ownerID)
	})
}

// cancelHandoffTimer stops a pending handoff timer for channelID, if any.
func (m *Manager) cancelHandoffTimer(channelID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if timer, running := m.handoffTimers[channelID]; running {
		timer.Stop()
		delete(m.handoffTimers, channelID)
	}
}

// runHandoff fires when a channel's owner-absence timer elapses. It
// re-validates the trigger condition against current state before acting,
// since the timer cannot be cancelled instantaneously from every code path.
func (m *Manager) runHandoff(channelID, absentOwnerID int64) {
	m.mu.Lock()
	delete(m.handoffTimers, channelID)
	m.mu.Unlock()

	party, exists, err := m.store.PartyByChannel(channelID)
	if err != nil {
		logger.Error("handoff: load party", "channel", channelID, "error", err)
		return
	}
	if !exists || party.OwnerID != absentOwnerID {
		return // ownership already changed since this timer was armed
	}

	channelIDStr := strconv.FormatInt(channelID, 10)
	members := m.membersInChannel(channelIDStr)
	if len(members) == 0 {
		return // channel emptied out; the cleanup timer owns this case
	}
	if containsUser(members, absentOwnerID) {
		return // owner returned
	}

	newOwnerID, err := m.pickHandoffSuccessor(members, absentOwnerID)
	if err != nil {
		logger.Error("handoff: pick successor", "channel", channelID, "error", err)
		return
	}

	if err := m.store.UpdateOwner(channelID, newOwnerID); err != nil {
		logger.Error("handoff: update owner", "channel", channelID, "error", err)
		return
	}
	if err := m.rewriteOverwrites(channelID, newOwnerID); err != nil {
		logger.Error("handoff: rewrite overwrites", "channel", channelID, "error", err)
		return
	}
	if _, err := m.session.ChannelMessageSend(channelIDStr, fmt.Sprintf(messages.NewOwner, newOwnerID)); err != nil {
		logger.Error("handoff: post notice", "channel", channelID, "error", err)
	}

	logger.Info("party handoff", "channel", channelID, "old_owner", absentOwnerID, "new_owner", newOwnerID)
}

// rewriteOverwrites recomputes and applies the full overwrite set for
// channelID against ownerID's current friends, the channel's active
// friends-of-friends sources, and the channel's manual party_overrides, per
// spec.md Ownership Rewrite. Sources survive the handoff - they belong to
// the channel, not the owner. In invite_only mode the owner's friend list is
// excluded entirely - only the owner and explicit party_overrides allow rows
// grant access. In public mode the owner's friend list is likewise excluded
// (default access already covers everyone) and the owner's globally-blocked
// users are loaded instead, as the auto-deny set.
func (m *Manager) rewriteOverwrites(channelID, ownerID int64) error {
	current, exists, err := m.store.PartyByChannel(channelID)
	if err != nil {
		return fmt.Errorf("load party %d: %w", channelID, err)
	}
	if !exists {
		return fmt.Errorf("party %d not found", channelID)
	}

	var friendIDs []int64
	if current.AccessMode != store.AccessModeInviteOnly && current.AccessMode != store.AccessModePublic {
		friendIDs, err = m.store.FriendIDs(ownerID)
		if err != nil {
			return fmt.Errorf("load friends for owner %d: %w", ownerID, err)
		}
	}
	sourceIDs, err := m.store.SourceIDsForChannel(channelID)
	if err != nil {
		return fmt.Errorf("load sources for channel %d: %w", channelID, err)
	}
	pendingInviteIDs, err := m.store.InviteIDsForChannel(channelID)
	if err != nil {
		return fmt.Errorf("load pending invites for channel %d: %w", channelID, err)
	}
	var blockedIDs []int64
	if current.AccessMode == store.AccessModePublic {
		blockedIDs, err = m.store.BlockIDs(ownerID)
		if err != nil {
			return fmt.Errorf("load blocked users for owner %d: %w", ownerID, err)
		}
	}
	overrides, err := m.store.OverridesForChannel(channelID)
	if err != nil {
		return fmt.Errorf("load overrides for channel %d: %w", channelID, err)
	}

	overwrites, err := buildRewriteOverwrites(m.store, m.guildID, ownerID, current.AccessMode, friendIDs, sourceIDs, pendingInviteIDs, blockedIDs, overrides)
	if err != nil {
		return fmt.Errorf("build overwrites for channel %d: %w", channelID, err)
	}

	channelIDStr := strconv.FormatInt(channelID, 10)
	_, err = m.session.ChannelEditComplex(channelIDStr, &discordgo.ChannelEdit{
		PermissionOverwrites: overwrites,
	})
	if err != nil {
		return fmt.Errorf("edit channel %d overwrites: %w", channelID, err)
	}
	return nil
}

func containsUser(members []string, userID int64) bool {
	return slices.Contains(members, strconv.FormatInt(userID, 10))
}

// pickHandoffSuccessor chooses the new owner from members present in the
// channel. It prefers a random pick among members the absent owner had
// marked as friends; if none of the present members are friends, it falls
// back to a random pick among all present members.
func (m *Manager) pickHandoffSuccessor(members []string, absentOwnerID int64) (int64, error) {
	friendIDs, err := m.store.FriendIDs(absentOwnerID)
	if err != nil {
		return 0, fmt.Errorf("load friends for absent owner %d: %w", absentOwnerID, err)
	}

	var friendMembers []string
	for _, member := range members {
		memberID, err := strconv.ParseInt(member, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse member id %q: %w", member, err)
		}
		if slices.Contains(friendIDs, memberID) {
			friendMembers = append(friendMembers, member)
		}
	}

	pool := members
	if len(friendMembers) > 0 {
		pool = friendMembers
	}

	newOwnerID, err := strconv.ParseInt(pool[rand.Intn(len(pool))], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse new owner id %q: %w", pool[0], err)
	}
	return newOwnerID, nil
}
