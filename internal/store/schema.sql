CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY   -- Discord user snowflake
);

CREATE TABLE IF NOT EXISTS relationships (
  granter_id    INTEGER NOT NULL REFERENCES users(id),
  grantee_id    INTEGER NOT NULL REFERENCES users(id),
  relation_type TEXT NOT NULL CHECK (relation_type IN ('friend','block')),
  created_at    INTEGER NOT NULL,
  PRIMARY KEY (granter_id, grantee_id)
);

CREATE INDEX IF NOT EXISTS idx_grantee ON relationships(grantee_id);

CREATE TABLE IF NOT EXISTS parties (
  channel_id  INTEGER PRIMARY KEY,   -- the party voice channel snowflake
  owner_id    INTEGER NOT NULL,      -- current owner snowflake
  created_at  INTEGER NOT NULL,
  access_mode TEXT NOT NULL DEFAULT 'friends_of_friends' CHECK (access_mode IN ('friends_of_friends','friends_only'))
);

CREATE TABLE IF NOT EXISTS party_overrides (
  channel_id INTEGER NOT NULL REFERENCES parties(channel_id),
  user_id    INTEGER NOT NULL,
  type       TEXT NOT NULL CHECK (type IN ('allow','deny')),
  PRIMARY KEY (channel_id, user_id)
);

-- Active friends-of-friends scan sources for a party: members who have been
-- present past the join delay and not yet past the leave grace. The
-- resulting allow-set (union of FriendIDs over every source) is never
-- stored - it's crawled at overwrite-rebuild time. See
-- internal/party/friendsoffriends.go.
CREATE TABLE IF NOT EXISTS party_sources (
  channel_id INTEGER NOT NULL REFERENCES parties(channel_id),
  user_id    INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  PRIMARY KEY (channel_id, user_id)
);

CREATE TABLE IF NOT EXISTS config (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
-- runtime rows set via /configure:
--   ('watch_channel_id',  '<snowflake>')
--   ('party_category_id', '<snowflake>')
