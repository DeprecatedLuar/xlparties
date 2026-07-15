// Package party manages the lifecycle of private per-user voice channels.
package party

import (
	"log"
	"strconv"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/store"
)

// Manager owns the party lifecycle: creation trigger today, handoff and
// cleanup in later phases.
type Manager struct {
	session *discordgo.Session
	store   *store.Store
	guildID string
}

// NewManager constructs a Manager. Call Register to start listening for the
// events that drive the lifecycle.
func NewManager(session *discordgo.Session, st *store.Store, guildID string) *Manager {
	return &Manager{session: session, store: st, guildID: guildID}
}

// Register attaches the voice-state-update handler that drives party
// creation.
func (m *Manager) Register() {
	m.session.AddHandler(m.onVoiceStateUpdate)
}

// WarnIfWatchChannelUnset logs a warning if no watch channel is configured
// yet, so the gap is visible at startup rather than discovered silently on
// the first join.
func (m *Manager) WarnIfWatchChannelUnset() {
	_, ok, err := m.store.GetConfig(store.ConfigKeyWatchChannel)
	if err != nil {
		log.Printf("check watch channel config: %v", err)
		return
	}
	if !ok {
		log.Println("watch_channel not configured — party creation disabled until /configure watch_channel is run")
	}
}

func (m *Manager) onVoiceStateUpdate(_ *discordgo.Session, v *discordgo.VoiceStateUpdate) {
	if v.ChannelID == "" {
		return // departure, not a join
	}

	watchChannelID, ok, err := m.store.GetConfig(store.ConfigKeyWatchChannel)
	if err != nil {
		log.Printf("load watch channel config: %v", err)
		return
	}
	if !ok || v.ChannelID != watchChannelID {
		return
	}

	ownerID, err := strconv.ParseInt(v.UserID, 10, 64)
	if err != nil {
		log.Printf("parse joining user id %q: %v", v.UserID, err)
		return
	}

	if err := m.spawnParty(ownerID); err != nil {
		log.Printf("spawn party for owner %d: %v", ownerID, err)
	}
}
