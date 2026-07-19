package party

import (
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/messages"
	"xlparties/internal/store"
)

// inviteCodeMaxAgeCap is the maximum age Discord accepts on a created
// invite (in seconds); ttl is clamped to this before creating the invite
// code, since InviteExpirySeconds is operator-configurable.
const inviteCodeMaxAgeCap = 604800 // 7 days, Discord's own invite MaxAge ceiling

// InviteOutcome reports what InviteToParty did, so the command handler can
// pick the right response for the caller.
type InviteOutcome int

const (
	// InviteGranted means a temp allow overwrite was set, a party_invites
	// row inserted, an expiry timer armed, and the target DMed a join link.
	InviteGranted InviteOutcome = iota
	// InviteAlreadyHasAccess means the target already had a standing allow
	// overwrite (owner, friend, or manual /party_allow); nothing was
	// granted or recorded, but the target was DMed a reminder.
	InviteAlreadyHasAccess
	// InviteRefused means the target has a standing deny overwrite, or is on
	// the owner's block list and the caller isn't the owner; nothing was
	// sent or changed.
	InviteRefused
)

// InviteToParty applies the /party_invite access-decision for targetID on
// channelID, keyed off the target's current channel overwrite: a standing
// allow overwrite is never touched (only DMed, so a friend is never
// revoked); a standing deny overwrite is always refused outright. The
// owner's block list also refuses the invite, but only for non-owner
// callers - the owner can invite someone they've blocked, since that's
// their own call to reverse. Otherwise a temp allow overwrite is granted,
// tracked with an expiry timer that is the explicit trigger to revoke it
// again.
func (m *Manager) InviteToParty(channelID, callerID, targetID int64) (InviteOutcome, error) {
	p, exists, err := m.store.PartyByChannel(channelID)
	if err != nil {
		return 0, fmt.Errorf("load party %d: %w", channelID, err)
	}
	if !exists {
		return 0, fmt.Errorf("party %d not found", channelID)
	}

	channelIDStr := strconv.FormatInt(channelID, 10)
	channel, err := m.session.Channel(channelIDStr)
	if err != nil {
		return 0, fmt.Errorf("load channel %d: %w", channelID, err)
	}

	targetIDStr := strconv.FormatInt(targetID, 10)
	for _, ow := range channel.PermissionOverwrites {
		if ow.Type != discordgo.PermissionOverwriteTypeMember || ow.ID != targetIDStr {
			continue
		}
		if ow.Allow&PartyChannelPermissions == PartyChannelPermissions {
			m.dmAlreadyHasAccess(targetID, callerID)
			return InviteAlreadyHasAccess, nil
		}
		if ow.Deny&PartyChannelPermissions == PartyChannelPermissions {
			return InviteRefused, nil
		}
		break
	}

	// In public mode a target with no deny overwrite already has default
	// access via @everyone: allow - granting a temp overwrite would be
	// redundant, so treat it the same as an existing standing allow.
	if p.AccessMode == store.AccessModePublic {
		m.dmAlreadyHasAccess(targetID, callerID)
		return InviteAlreadyHasAccess, nil
	}

	if callerID != p.OwnerID {
		blocked, err := m.store.IsBlocked(p.OwnerID, targetID)
		if err != nil {
			return 0, fmt.Errorf("check block list for owner %d: %w", p.OwnerID, err)
		}
		if blocked {
			return InviteRefused, nil
		}
	}

	if err := m.session.ChannelPermissionSet(channelIDStr, targetIDStr, discordgo.PermissionOverwriteTypeMember, int64(PartyChannelPermissions), 0); err != nil {
		return 0, fmt.Errorf("grant temp invite overwrite: %w", err)
	}
	expiresAt := time.Now().Add(m.inviteExpiry).Unix()
	if err := m.store.AddInvite(channelID, targetID, expiresAt); err != nil {
		return 0, fmt.Errorf("record invite: %w", err)
	}
	m.startInviteExpiryTimer(channelID, targetID, m.inviteExpiry)
	m.dmInviteGranted(targetID, callerID, channelIDStr)

	logger.Info("party invite granted", "channel", channelID, "caller", callerID, "target", targetID)
	return InviteGranted, nil
}

// dmAlreadyHasAccess best-effort DMs targetID that callerID tried to invite
// them but they already have standing access, with no join link needed
// since they can already see and join the channel directly. DM failures are
// logged, not surfaced to the caller.
func (m *Manager) dmAlreadyHasAccess(targetID, callerID int64) {
	channel, err := m.session.UserChannelCreate(strconv.FormatInt(targetID, 10))
	if err != nil {
		logger.Error("party invite: could not open DM", "target", targetID, "error", err)
		return
	}
	msg := fmt.Sprintf(messages.PartyInviteDMAlreadyHasAccess, messages.RandomGreeting(), callerID)
	if _, err := m.session.ChannelMessageSend(channel.ID, msg); err != nil {
		logger.Error("party invite: could not DM already-has-access notice", "target", targetID, "error", err)
	}
}

// dmInviteGranted best-effort DMs targetID a one-click join link for
// channelIDStr, created as a short-lived native Discord invite (a plain
// channel link would not work since the target has no standing VIEW_CHANNEL
// overwrite yet). DM/invite-creation failures are logged, not surfaced to
// the caller - the overwrite grant itself already succeeded.
func (m *Manager) dmInviteGranted(targetID, callerID int64, channelIDStr string) {
	maxAge := int(m.inviteExpiry.Seconds())
	if maxAge <= 0 || maxAge > inviteCodeMaxAgeCap {
		maxAge = inviteCodeMaxAgeCap
	}

	invite, err := m.session.ChannelInviteCreate(channelIDStr, discordgo.Invite{
		MaxAge: maxAge,
		Unique: true,
	})
	if err != nil {
		logger.Error("party invite: could not create invite code", "channel", channelIDStr, "target", targetID, "error", err)
		return
	}

	channel, err := m.session.UserChannelCreate(strconv.FormatInt(targetID, 10))
	if err != nil {
		logger.Error("party invite: could not open DM", "target", targetID, "error", err)
		return
	}
	msg := fmt.Sprintf(messages.PartyInviteDMBody, messages.RandomGreeting(), callerID, "https://discord.gg/"+invite.Code)
	if _, err := m.session.ChannelMessageSend(channel.ID, msg); err != nil {
		logger.Error("party invite: could not DM join link", "target", targetID, "error", err)
	}
}

// startInviteExpiryTimer arms the TTL timer for (channelID, userID)'s
// pending invite if one isn't already running.
func (m *Manager) startInviteExpiryTimer(channelID, userID int64, ttl time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.inviteTimers[channelID]; !ok {
		m.inviteTimers[channelID] = make(map[int64]*time.Timer)
	}
	if timer, running := m.inviteTimers[channelID][userID]; running {
		timer.Stop()
	}
	m.inviteTimers[channelID][userID] = time.AfterFunc(ttl, func() {
		m.runInviteExpiry(channelID, userID)
	})
}

// cancelInviteExpiryTimer stops a pending invite-expiry timer for
// (channelID, userID), if any.
func (m *Manager) cancelInviteExpiryTimer(channelID, userID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	timers, ok := m.inviteTimers[channelID]
	if !ok {
		return
	}
	if timer, running := timers[userID]; running {
		timer.Stop()
		delete(timers, userID)
	}
}

// cancelInviteTimersForChannel stops and drops every pending invite-expiry
// timer for channelID, on party cleanup.
func (m *Manager) cancelInviteTimersForChannel(channelID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, timer := range m.inviteTimers[channelID] {
		timer.Stop()
	}
	delete(m.inviteTimers, channelID)
}

// runInviteExpiry fires when a pending invite's TTL elapses. It re-validates
// against current state before acting: the target may already have joined
// (onJoinChannel should have caught that and cancelled this timer, guarded
// anyway), or the party may have been deleted.
func (m *Manager) runInviteExpiry(channelID, userID int64) {
	m.mu.Lock()
	if timers, ok := m.inviteTimers[channelID]; ok {
		delete(timers, userID)
	}
	m.mu.Unlock()

	if _, exists, err := m.store.PartyByChannel(channelID); err != nil {
		logger.Error("invite expiry: load party", "channel", channelID, "error", err)
		return
	} else if !exists {
		return
	}

	m.revokeInvite(channelID, userID)
	logger.Info("invite expiry: revoked unused invite", "channel", channelID, "user", userID)
}

// ClearPendingInvite drops any pending /party_invite row and expiry timer
// for (channelID, userID), without touching the channel overwrite. Called
// by /party_ban and /party_block so a subsequent invite-expiry fire doesn't
// clobber the deny overwrite those commands just set.
func (m *Manager) ClearPendingInvite(channelID, userID int64) error {
	m.cancelInviteExpiryTimer(channelID, userID)
	return m.store.RemoveInvite(channelID, userID)
}

// consumePendingInvite revokes a pending /party_invite grant for (channelID,
// userID) the moment the invitee joins. The party_invites row existing is
// the explicit signal that the current overwrite is a temp invite grant and
// not a standing allow - friend and owner overwrites never create a row, so
// this never touches those.
func (m *Manager) consumePendingInvite(channelID, userID int64) {
	inviteIDs, err := m.store.InviteIDsForChannel(channelID)
	if err != nil {
		logger.Error("consume pending invite: load invites for channel", "channel", channelID, "error", err)
		return
	}
	if !slices.Contains(inviteIDs, userID) {
		return
	}
	m.cancelInviteExpiryTimer(channelID, userID)
	m.revokeInvite(channelID, userID)
	logger.Info("party invite consumed", "channel", channelID, "user", userID)
}

// revokeInvite removes the temp allow overwrite and party_invites row for
// (channelID, userID). Shared by the join handler (invite consumed) and
// runInviteExpiry (invite unused).
func (m *Manager) revokeInvite(channelID, userID int64) {
	channelIDStr := strconv.FormatInt(channelID, 10)
	userIDStr := strconv.FormatInt(userID, 10)
	if err := m.session.ChannelPermissionDelete(channelIDStr, userIDStr); err != nil {
		logger.Error("revoke invite: delete channel permission", "channel", channelID, "user", userID, "error", err)
	}
	if err := m.store.RemoveInvite(channelID, userID); err != nil {
		logger.Error("revoke invite: remove invite row", "channel", channelID, "user", userID, "error", err)
	}
}
