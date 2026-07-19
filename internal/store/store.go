// Package store wraps the SQLite persistence layer: schema, and all query
// methods used by the rest of the bot.
package store

import (
	"database/sql"
	_ "embed"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// Config keys set at runtime via /configure, shared between the writer
// (commands package) and readers (party package).
const (
	ConfigKeyWatchChannel = "watch_channel_id"
	ConfigKeyCategory     = "party_category_id"
)

// Access modes stored on the parties table.
const (
	AccessModeFriendsOfFriends = "friends_of_friends"
	AccessModeFriendsOnly      = "friends_only"
	AccessModeInviteOnly       = "invite_only"
)

// Store wraps the database connection and exposes all query methods.
type Store struct {
	db *sql.DB
}

// Party is a row from the parties table.
type Party struct {
	ChannelID  int64
	OwnerID    int64
	CreatedAt  int64
	AccessMode string
}

// Override is a row from the party_overrides table.
type Override struct {
	ChannelID int64
	UserID    int64
	Type      string // "allow" or "deny"
}

// Open opens (creating if absent) the SQLite database at path, applies the
// schema idempotently, and enables WAL + foreign key enforcement.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable WAL: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign_keys: %w", err)
	}
	if _, err := db.Exec(schemaSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	if err := migrateSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) upsertUser(userID int64) error {
	_, err := s.db.Exec(`INSERT INTO users (id) VALUES (?) ON CONFLICT (id) DO NOTHING`, userID)
	if err != nil {
		return fmt.Errorf("upsert user %d: %w", userID, err)
	}
	return nil
}

// --- relationships ---

// UpsertFriend records that granterID allows granteeID to see and join
// granterID's party by default. Replaces any existing block edge.
func (s *Store) UpsertFriend(granterID, granteeID int64) error {
	return s.upsertRelationship(granterID, granteeID, "friend")
}

// UpsertBlock records that granterID denies granteeID access. Replaces any
// existing friend edge.
func (s *Store) UpsertBlock(granterID, granteeID int64) error {
	return s.upsertRelationship(granterID, granteeID, "block")
}

func (s *Store) upsertRelationship(granterID, granteeID int64, relationType string) error {
	if err := s.upsertUser(granterID); err != nil {
		return err
	}
	if err := s.upsertUser(granteeID); err != nil {
		return err
	}
	_, err := s.db.Exec(`
		INSERT INTO relationships (granter_id, grantee_id, relation_type, created_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (granter_id, grantee_id)
		DO UPDATE SET relation_type = excluded.relation_type, created_at = excluded.created_at
	`, granterID, granteeID, relationType, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("upsert relationship (%d,%d,%s): %w", granterID, granteeID, relationType, err)
	}
	return nil
}

// RemoveFriend deletes the (granterID, granteeID, 'friend') edge, if present.
func (s *Store) RemoveFriend(granterID, granteeID int64) error {
	return s.removeRelationship(granterID, granteeID, "friend")
}

// RemoveBlock deletes the (granterID, granteeID, 'block') edge, if present.
func (s *Store) RemoveBlock(granterID, granteeID int64) error {
	return s.removeRelationship(granterID, granteeID, "block")
}

func (s *Store) removeRelationship(granterID, granteeID int64, relationType string) error {
	_, err := s.db.Exec(`
		DELETE FROM relationships WHERE granter_id = ? AND grantee_id = ? AND relation_type = ?
	`, granterID, granteeID, relationType)
	if err != nil {
		return fmt.Errorf("remove relationship (%d,%d,%s): %w", granterID, granteeID, relationType, err)
	}
	return nil
}

// IsFriend reports whether granterID already has a friend edge to granteeID.
func (s *Store) IsFriend(granterID, granteeID int64) (bool, error) {
	var exists bool
	err := s.db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM relationships
			WHERE granter_id = ? AND grantee_id = ? AND relation_type = 'friend'
		)
	`, granterID, granteeID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check friend (%d,%d): %w", granterID, granteeID, err)
	}
	return exists, nil
}

// IsBlocked reports whether granterID has blocked granteeID.
func (s *Store) IsBlocked(granterID, granteeID int64) (bool, error) {
	var exists bool
	err := s.db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM relationships
			WHERE granter_id = ? AND grantee_id = ? AND relation_type = 'block'
		)
	`, granterID, granteeID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check blocked (%d,%d): %w", granterID, granteeID, err)
	}
	return exists, nil
}

// FriendIDs returns the ids of every user ownerID has marked as a friend.
func (s *Store) FriendIDs(ownerID int64) ([]int64, error) {
	return s.relationshipIDs(ownerID, "friend")
}

// BlockIDs returns the ids of every user ownerID has blocked.
func (s *Store) BlockIDs(ownerID int64) ([]int64, error) {
	return s.relationshipIDs(ownerID, "block")
}

func (s *Store) relationshipIDs(granterID int64, relationType string) ([]int64, error) {
	rows, err := s.db.Query(`
		SELECT grantee_id FROM relationships WHERE granter_id = ? AND relation_type = ?
	`, granterID, relationType)
	if err != nil {
		return nil, fmt.Errorf("query %s ids for %d: %w", relationType, granterID, err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan %s id: %w", relationType, err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// --- config ---

// GetConfig returns the value stored under key, and whether it was present.
func (s *Store) GetConfig(key string) (string, bool, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM config WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get config %q: %w", key, err)
	}
	return value, true, nil
}

// SetConfig upserts the value stored under key.
func (s *Store) SetConfig(key, value string) error {
	_, err := s.db.Exec(`
		INSERT INTO config (key, value) VALUES (?, ?)
		ON CONFLICT (key) DO UPDATE SET value = excluded.value
	`, key, value)
	if err != nil {
		return fmt.Errorf("set config %q: %w", key, err)
	}
	return nil
}

// --- parties ---

// InsertParty records a newly created party channel.
func (s *Store) InsertParty(channelID, ownerID int64) error {
	_, err := s.db.Exec(`
		INSERT INTO parties (channel_id, owner_id, created_at) VALUES (?, ?, ?)
	`, channelID, ownerID, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("insert party %d: %w", channelID, err)
	}
	return nil
}

// DeleteParty removes the party row for channelID.
func (s *Store) DeleteParty(channelID int64) error {
	_, err := s.db.Exec(`DELETE FROM parties WHERE channel_id = ?`, channelID)
	if err != nil {
		return fmt.Errorf("delete party %d: %w", channelID, err)
	}
	return nil
}

// PartyByOwner returns the active party owned by ownerID, if any.
func (s *Store) PartyByOwner(ownerID int64) (*Party, bool, error) {
	return s.queryOneParty(`SELECT channel_id, owner_id, created_at, access_mode FROM parties WHERE owner_id = ?`, ownerID)
}

// PartyByChannel returns the party row for channelID, if any.
func (s *Store) PartyByChannel(channelID int64) (*Party, bool, error) {
	return s.queryOneParty(`SELECT channel_id, owner_id, created_at, access_mode FROM parties WHERE channel_id = ?`, channelID)
}

func (s *Store) queryOneParty(query string, arg int64) (*Party, bool, error) {
	var p Party
	err := s.db.QueryRow(query, arg).Scan(&p.ChannelID, &p.OwnerID, &p.CreatedAt, &p.AccessMode)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("query party: %w", err)
	}
	return &p, true, nil
}

// AllParties returns every party row, for the startup sweep.
func (s *Store) AllParties() ([]Party, error) {
	rows, err := s.db.Query(`SELECT channel_id, owner_id, created_at, access_mode FROM parties`)
	if err != nil {
		return nil, fmt.Errorf("query all parties: %w", err)
	}
	defer rows.Close()

	var parties []Party
	for rows.Next() {
		var p Party
		if err := rows.Scan(&p.ChannelID, &p.OwnerID, &p.CreatedAt, &p.AccessMode); err != nil {
			return nil, fmt.Errorf("scan party: %w", err)
		}
		parties = append(parties, p)
	}
	return parties, rows.Err()
}

// UpdateOwner changes the owner of record for channelID, on handoff.
func (s *Store) UpdateOwner(channelID, newOwnerID int64) error {
	_, err := s.db.Exec(`UPDATE parties SET owner_id = ? WHERE channel_id = ?`, newOwnerID, channelID)
	if err != nil {
		return fmt.Errorf("update owner for party %d: %w", channelID, err)
	}
	return nil
}

// UpdateAccessMode changes channelID's access_mode, on /party_mode. The
// CHECK constraint on the column is the only validation of mode; callers are
// expected to pass one of the AccessMode* constants.
func (s *Store) UpdateAccessMode(channelID int64, mode string) error {
	_, err := s.db.Exec(`UPDATE parties SET access_mode = ? WHERE channel_id = ?`, mode, channelID)
	if err != nil {
		return fmt.Errorf("update access_mode for party %d: %w", channelID, err)
	}
	return nil
}

// --- party_pending_creations ---

// PendingCreation is a row from the party_pending_creations table.
type PendingCreation struct {
	OwnerID   int64
	CreatedAt int64
}

// InsertPendingCreation marks ownerID's party creation as in flight, called
// right before the Discord channel-create call so a crash mid-create leaves
// a durable trace (upserts created_at so a retried creation isn't blocked by
// a leftover row from an interrupted prior attempt for the same owner).
func (s *Store) InsertPendingCreation(ownerID int64) error {
	_, err := s.db.Exec(`
		INSERT INTO party_pending_creations (owner_id, created_at) VALUES (?, ?)
		ON CONFLICT (owner_id) DO UPDATE SET created_at = excluded.created_at
	`, ownerID, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("insert pending creation for owner %d: %w", ownerID, err)
	}
	return nil
}

// DeletePendingCreation clears the in-flight marker for ownerID, called once
// spawnParty finishes (success or failure).
func (s *Store) DeletePendingCreation(ownerID int64) error {
	_, err := s.db.Exec(`DELETE FROM party_pending_creations WHERE owner_id = ?`, ownerID)
	if err != nil {
		return fmt.Errorf("delete pending creation for owner %d: %w", ownerID, err)
	}
	return nil
}

// AllPendingCreations returns every in-flight creation marker, for the
// startup reconciliation that looks for channels leaked by a crash mid-create.
func (s *Store) AllPendingCreations() ([]PendingCreation, error) {
	rows, err := s.db.Query(`SELECT owner_id, created_at FROM party_pending_creations`)
	if err != nil {
		return nil, fmt.Errorf("query all pending creations: %w", err)
	}
	defer rows.Close()

	var pending []PendingCreation
	for rows.Next() {
		var p PendingCreation
		if err := rows.Scan(&p.OwnerID, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan pending creation: %w", err)
		}
		pending = append(pending, p)
	}
	return pending, rows.Err()
}

// --- party_overrides ---

// UpsertOverride records a manual /party_allow or /party_block decision.
func (s *Store) UpsertOverride(channelID, userID int64, overrideType string) error {
	_, err := s.db.Exec(`
		INSERT INTO party_overrides (channel_id, user_id, type) VALUES (?, ?, ?)
		ON CONFLICT (channel_id, user_id) DO UPDATE SET type = excluded.type
	`, channelID, userID, overrideType)
	if err != nil {
		return fmt.Errorf("upsert override (%d,%d,%s): %w", channelID, userID, overrideType, err)
	}
	return nil
}

// DeleteOverridesForChannel removes all manual overrides for channelID, on
// party cleanup.
func (s *Store) DeleteOverridesForChannel(channelID int64) error {
	_, err := s.db.Exec(`DELETE FROM party_overrides WHERE channel_id = ?`, channelID)
	if err != nil {
		return fmt.Errorf("delete overrides for channel %d: %w", channelID, err)
	}
	return nil
}

// OverridesForChannel returns all manual overrides for channelID.
func (s *Store) OverridesForChannel(channelID int64) ([]Override, error) {
	rows, err := s.db.Query(`SELECT channel_id, user_id, type FROM party_overrides WHERE channel_id = ?`, channelID)
	if err != nil {
		return nil, fmt.Errorf("query overrides for channel %d: %w", channelID, err)
	}
	defer rows.Close()

	var overrides []Override
	for rows.Next() {
		var o Override
		if err := rows.Scan(&o.ChannelID, &o.UserID, &o.Type); err != nil {
			return nil, fmt.Errorf("scan override: %w", err)
		}
		overrides = append(overrides, o)
	}
	return overrides, rows.Err()
}

// --- party_sources ---

// AddSource records userID as an active friends-of-friends scan source for
// channelID (their friends contribute to the party's allow-set).
func (s *Store) AddSource(channelID, userID int64) error {
	_, err := s.db.Exec(`
		INSERT INTO party_sources (channel_id, user_id, created_at) VALUES (?, ?, ?)
		ON CONFLICT (channel_id, user_id) DO NOTHING
	`, channelID, userID, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("add source (%d,%d): %w", channelID, userID, err)
	}
	return nil
}

// RemoveSource drops userID as a scan source for channelID.
func (s *Store) RemoveSource(channelID, userID int64) error {
	_, err := s.db.Exec(`DELETE FROM party_sources WHERE channel_id = ? AND user_id = ?`, channelID, userID)
	if err != nil {
		return fmt.Errorf("remove source (%d,%d): %w", channelID, userID, err)
	}
	return nil
}

// RemoveSourcesForChannel drops all scan sources for channelID, on party
// cleanup or a switch to friends-only mode.
func (s *Store) RemoveSourcesForChannel(channelID int64) error {
	_, err := s.db.Exec(`DELETE FROM party_sources WHERE channel_id = ?`, channelID)
	if err != nil {
		return fmt.Errorf("remove sources for channel %d: %w", channelID, err)
	}
	return nil
}

// SourceIDsForChannel returns the ids of every active scan source for
// channelID.
func (s *Store) SourceIDsForChannel(channelID int64) ([]int64, error) {
	rows, err := s.db.Query(`SELECT user_id FROM party_sources WHERE channel_id = ?`, channelID)
	if err != nil {
		return nil, fmt.Errorf("query sources for channel %d: %w", channelID, err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan source id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ChannelsForSource returns the ids of every party channel where userID is
// an active friends-of-friends scan source.
func (s *Store) ChannelsForSource(userID int64) ([]int64, error) {
	rows, err := s.db.Query(`SELECT channel_id FROM party_sources WHERE user_id = ?`, userID)
	if err != nil {
		return nil, fmt.Errorf("query channels for source %d: %w", userID, err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan channel id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// --- party_invites ---

// Invite is a row from the party_invites table.
type Invite struct {
	ChannelID int64
	UserID    int64
	ExpiresAt int64
}

// AddInvite records a pending /party_invite grant for (channelID, userID),
// refreshing expiresAt if a row already exists (re-inviting resets the TTL).
func (s *Store) AddInvite(channelID, userID, expiresAt int64) error {
	_, err := s.db.Exec(`
		INSERT INTO party_invites (channel_id, user_id, expires_at) VALUES (?, ?, ?)
		ON CONFLICT (channel_id, user_id) DO UPDATE SET expires_at = excluded.expires_at
	`, channelID, userID, expiresAt)
	if err != nil {
		return fmt.Errorf("add invite (%d,%d): %w", channelID, userID, err)
	}
	return nil
}

// RemoveInvite deletes the pending invite row for (channelID, userID), if any.
func (s *Store) RemoveInvite(channelID, userID int64) error {
	_, err := s.db.Exec(`DELETE FROM party_invites WHERE channel_id = ? AND user_id = ?`, channelID, userID)
	if err != nil {
		return fmt.Errorf("remove invite (%d,%d): %w", channelID, userID, err)
	}
	return nil
}

// InviteIDsForChannel returns the ids of every user with a pending invite for
// channelID.
func (s *Store) InviteIDsForChannel(channelID int64) ([]int64, error) {
	rows, err := s.db.Query(`SELECT user_id FROM party_invites WHERE channel_id = ?`, channelID)
	if err != nil {
		return nil, fmt.Errorf("query invites for channel %d: %w", channelID, err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan invite user id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// RemoveInvitesForChannel drops all pending invites for channelID, on party
// cleanup.
func (s *Store) RemoveInvitesForChannel(channelID int64) error {
	_, err := s.db.Exec(`DELETE FROM party_invites WHERE channel_id = ?`, channelID)
	if err != nil {
		return fmt.Errorf("remove invites for channel %d: %w", channelID, err)
	}
	return nil
}

// AllInvites returns every pending invite row, for the startup sweep.
func (s *Store) AllInvites() ([]Invite, error) {
	rows, err := s.db.Query(`SELECT channel_id, user_id, expires_at FROM party_invites`)
	if err != nil {
		return nil, fmt.Errorf("query all invites: %w", err)
	}
	defer rows.Close()

	var invites []Invite
	for rows.Next() {
		var inv Invite
		if err := rows.Scan(&inv.ChannelID, &inv.UserID, &inv.ExpiresAt); err != nil {
			return nil, fmt.Errorf("scan invite: %w", err)
		}
		invites = append(invites, inv)
	}
	return invites, rows.Err()
}
