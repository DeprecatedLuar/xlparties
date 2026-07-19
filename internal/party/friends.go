package party

import (
	"errors"
	"fmt"
)

// RewriteAffectedChannels recomputes overwrites for every active party
// channel userID's friend list can influence: a channel userID owns, and
// any channel where userID is a friends-of-friends scan source. Called after
// a relationship change so live channels reflect it immediately instead of
// only at the next handoff.
func (m *Manager) RewriteAffectedChannels(userID int64) error {
	var errs []error

	if owned, found, err := m.store.PartyByOwner(userID); err != nil {
		errs = append(errs, fmt.Errorf("lookup owned party for %d: %w", userID, err))
	} else if found {
		if err := m.rewriteOverwrites(owned.ChannelID, owned.OwnerID); err != nil {
			errs = append(errs, fmt.Errorf("rewrite owned channel %d: %w", owned.ChannelID, err))
		}
	}

	sourceChannels, err := m.store.ChannelsForSource(userID)
	if err != nil {
		errs = append(errs, fmt.Errorf("lookup source channels for %d: %w", userID, err))
		return errors.Join(errs...)
	}
	for _, channelID := range sourceChannels {
		sourceParty, found, err := m.store.PartyByChannel(channelID)
		if err != nil {
			errs = append(errs, fmt.Errorf("lookup party for source channel %d: %w", channelID, err))
			continue
		}
		if !found {
			continue
		}
		if err := m.rewriteOverwrites(channelID, sourceParty.OwnerID); err != nil {
			errs = append(errs, fmt.Errorf("rewrite source channel %d: %w", channelID, err))
		}
	}

	return errors.Join(errs...)
}
