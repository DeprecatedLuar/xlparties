// Package party manages the lifecycle of private per-user voice channels.
package party

import (
	"strconv"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/store"
)

// Manager owns the party lifecycle: creation, ownership handoff, and
// empty-channel cleanup.
type Manager struct {
	session *discordgo.Session
	store   *store.Store
	guildID string

	emptyCleanup        time.Duration
	ownerAbsenceHandoff time.Duration

	mu            sync.Mutex
	handoffTimers map[int64]*time.Timer
	cleanupTimers map[int64]*time.Timer

	// fofJoinTimers/fofLeaveTimers drive the friends-of-friends scan: keyed
	// by channel id then member id. See friendsoffriends.go.
	fofJoinTimers  map[int64]map[int64]*time.Timer
	fofLeaveTimers map[int64]map[int64]*time.Timer
}

// NewManager constructs a Manager. Call Register to start listening for the
// events that drive the lifecycle.
func NewManager(session *discordgo.Session, st *store.Store, guildID string, emptyCleanup, ownerAbsenceHandoff time.Duration) *Manager {
	return &Manager{
		session:             session,
		store:               st,
		guildID:             guildID,
		emptyCleanup:        emptyCleanup,
		ownerAbsenceHandoff: ownerAbsenceHandoff,
		handoffTimers:       make(map[int64]*time.Timer),
		cleanupTimers:       make(map[int64]*time.Timer),
		fofJoinTimers:       make(map[int64]map[int64]*time.Timer),
		fofLeaveTimers:      make(map[int64]map[int64]*time.Timer),
	}
}

// Register attaches the voice-state-update handler that drives party
// creation.
func (m *Manager) Register() {
	m.session.AddHandler(m.onVoiceStateUpdate)
}

// WarnIfUnconfigured logs a warning for any required config (watch channel,
// party category) that is missing at startup, so the gap is visible
// immediately rather than discovered silently on the first join.
func (m *Manager) WarnIfUnconfigured() {
	_, ok, err := m.store.GetConfig(store.ConfigKeyWatchChannel)
	if err != nil {
		logger.Error("check watch channel config", "error", err)
	} else if !ok {
		logger.Warn("watch_channel not configured — party creation disabled until /configure watch_channel is run")
	}

	_, ok, err = m.store.GetConfig(store.ConfigKeyCategory)
	if err != nil {
		logger.Error("check party category config", "error", err)
	} else if !ok {
		logger.Warn("party_category not configured — party creation will fail until /configure category is run")
	}
}

func (m *Manager) onVoiceStateUpdate(_ *discordgo.Session, v *discordgo.VoiceStateUpdate) {
	var beforeChannelID string
	if v.BeforeUpdate != nil {
		beforeChannelID = v.BeforeUpdate.ChannelID
	}
	if beforeChannelID == v.ChannelID {
		return // mute/deaf/etc. update, no channel change
	}

	if beforeChannelID != "" {
		m.onLeaveChannel(beforeChannelID, v.UserID)
	}
	if v.ChannelID != "" {
		m.onJoinChannel(v.ChannelID, v.UserID)
	}
}

// onJoinChannel handles both triggers a channel join can fire: spawning a
// party if the channel is the configured watch channel, and cancelling any
// pending cleanup/handoff timer if the channel is an existing party channel.
func (m *Manager) onJoinChannel(channelID, userID string) {
	watchChannelID, ok, err := m.store.GetConfig(store.ConfigKeyWatchChannel)
	if err != nil {
		logger.Error("load watch channel config", "error", err)
	} else if ok && channelID == watchChannelID {
		ownerID, err := strconv.ParseInt(userID, 10, 64)
		if err != nil {
			logger.Error("parse joining user id", "user_id", userID, "error", err)
		} else if err := m.spawnParty(ownerID); err != nil {
			logger.Error("spawn party", "owner", ownerID, "error", err)
		}
	}

	channelIDInt, err := strconv.ParseInt(channelID, 10, 64)
	if err != nil {
		logger.Error("parse channel id", "channel_id", channelID, "error", err)
		return
	}
	party, exists, err := m.store.PartyByChannel(channelIDInt)
	if err != nil {
		logger.Error("check party for channel", "channel", channelIDInt, "error", err)
		return
	}
	if !exists {
		return
	}

	m.cancelCleanupTimer(channelIDInt)
	if strconv.FormatInt(party.OwnerID, 10) == userID {
		m.cancelHandoffTimer(channelIDInt)
	} else if party.AccessMode == store.AccessModeFriendsOfFriends {
		userIDInt, err := strconv.ParseInt(userID, 10, 64)
		if err != nil {
			logger.Error("parse joining user id", "user_id", userID, "error", err)
			return
		}
		m.cancelFoFLeaveTimer(channelIDInt, userIDInt)
		m.startFoFJoinTimer(channelIDInt, userIDInt)
	}
}

// onLeaveChannel reacts to a member leaving a party channel: starts the
// empty-cleanup grace timer if the channel is now empty, or the
// owner-absence handoff timer if the owner left while others remain.
func (m *Manager) onLeaveChannel(channelID, userID string) {
	channelIDInt, err := strconv.ParseInt(channelID, 10, 64)
	if err != nil {
		logger.Error("parse channel id", "channel_id", channelID, "error", err)
		return
	}
	party, exists, err := m.store.PartyByChannel(channelIDInt)
	if err != nil {
		logger.Error("check party for channel", "channel", channelIDInt, "error", err)
		return
	}
	if !exists {
		return
	}

	if party.AccessMode == store.AccessModeFriendsOfFriends && strconv.FormatInt(party.OwnerID, 10) != userID {
		userIDInt, err := strconv.ParseInt(userID, 10, 64)
		if err != nil {
			logger.Error("parse leaving user id", "user_id", userID, "error", err)
		} else {
			m.cancelFoFJoinTimer(channelIDInt, userIDInt)
			m.startFoFLeaveTimer(channelIDInt, userIDInt)
		}
	}

	members := m.membersInChannel(channelID)
	if len(members) == 0 {
		m.startCleanupTimer(channelIDInt)
		return
	}
	if strconv.FormatInt(party.OwnerID, 10) == userID {
		m.startHandoffTimer(channelIDInt, party.OwnerID)
	}
}

// membersInChannel returns the ids of non-bot members currently connected to
// channelID, read from the library's voice-state cache. Bots (e.g. music
// bots) are excluded: they are not eligible for ownership handoff and must
// not count as occupants when deciding whether a channel is empty.
func (m *Manager) membersInChannel(channelID string) []string {
	guild, err := m.session.State.Guild(m.guildID)
	if err != nil {
		logger.Error("load guild voice state", "error", err)
		return nil
	}
	var members []string
	for _, vs := range guild.VoiceStates {
		if vs.ChannelID != channelID {
			continue
		}
		if vs.Member != nil && vs.Member.User != nil && vs.Member.User.Bot {
			continue
		}
		members = append(members, vs.UserID)
	}
	return members
}
