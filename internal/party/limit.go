package party

import (
	"fmt"
	"strconv"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
)

// minUserLimit and maxUserLimit are Discord's own voice channel user_limit
// bounds; 0 means unlimited.
const (
	minUserLimit = 0
	maxUserLimit = 99
)

// SetUserLimit validates limit and applies it to channelID's live voice
// channel. The limit is not persisted - Discord itself is the source of
// truth for the current channel; only /party_preset's saved value lives in
// the store.
//
// discordgo.ChannelEdit's UserLimit field is tagged `omitempty`, so passing
// it through ChannelEditComplex would silently drop a limit of 0 (unlimited)
// instead of sending it. Sending the raw payload directly is the only way to
// actually clear the limit back to unlimited.
func (m *Manager) SetUserLimit(channelID int64, limit int) error {
	if limit < minUserLimit || limit > maxUserLimit {
		return fmt.Errorf("user limit %d out of range [%d, %d]", limit, minUserLimit, maxUserLimit)
	}

	channelIDStr := strconv.FormatInt(channelID, 10)
	payload := struct {
		UserLimit int `json:"user_limit"`
	}{UserLimit: limit}
	_, err := m.session.RequestWithBucketID("PATCH", discordgo.EndpointChannel(channelIDStr), payload, discordgo.EndpointChannel(channelIDStr))
	if err != nil {
		return fmt.Errorf("set user limit for channel %d: %w", channelID, err)
	}

	logger.Info("party user limit changed", "channel", channelID, "limit", limit)
	return nil
}
