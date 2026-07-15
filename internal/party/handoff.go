package party

import (
	"fmt"
	"log"
	"math/rand"
	"slices"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
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
		log.Printf("handoff: load party %d: %v", channelID, err)
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

	newOwnerIDStr := members[rand.Intn(len(members))]
	newOwnerID, err := strconv.ParseInt(newOwnerIDStr, 10, 64)
	if err != nil {
		log.Printf("handoff: parse new owner id %q: %v", newOwnerIDStr, err)
		return
	}

	if err := m.store.UpdateOwner(channelID, newOwnerID); err != nil {
		log.Printf("handoff: update owner for channel %d: %v", channelID, err)
		return
	}
	if err := m.rewriteOverwrites(channelID, newOwnerID); err != nil {
		log.Printf("handoff: rewrite overwrites for channel %d: %v", channelID, err)
		return
	}
	if _, err := m.session.ChannelMessageSend(channelIDStr, fmt.Sprintf("<@%d> is now the owner of this party.", newOwnerID)); err != nil {
		log.Printf("handoff: post notice in channel %d: %v", channelID, err)
	}

	log.Printf("party handoff: channel=%d old_owner=%d new_owner=%d", channelID, absentOwnerID, newOwnerID)
}

// rewriteOverwrites recomputes and applies the full overwrite set for
// channelID against ownerID's current friends and the channel's manual
// party_overrides, per spec.md Ownership Rewrite.
func (m *Manager) rewriteOverwrites(channelID, ownerID int64) error {
	friendIDs, err := m.store.FriendIDs(ownerID)
	if err != nil {
		return fmt.Errorf("load friends for owner %d: %w", ownerID, err)
	}
	overrides, err := m.store.OverridesForChannel(channelID)
	if err != nil {
		return fmt.Errorf("load overrides for channel %d: %w", channelID, err)
	}

	channelIDStr := strconv.FormatInt(channelID, 10)
	_, err = m.session.ChannelEditComplex(channelIDStr, &discordgo.ChannelEdit{
		PermissionOverwrites: buildRewriteOverwrites(m.guildID, ownerID, friendIDs, overrides),
	})
	if err != nil {
		return fmt.Errorf("edit channel %d overwrites: %w", channelID, err)
	}
	return nil
}

func containsUser(members []string, userID int64) bool {
	return slices.Contains(members, strconv.FormatInt(userID, 10))
}
