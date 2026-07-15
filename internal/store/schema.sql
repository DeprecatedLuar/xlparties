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
  channel_id INTEGER PRIMARY KEY,   -- the party voice channel snowflake
  owner_id   INTEGER NOT NULL,      -- current owner snowflake
  created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS party_overrides (
  channel_id INTEGER NOT NULL REFERENCES parties(channel_id),
  user_id    INTEGER NOT NULL,
  type       TEXT NOT NULL CHECK (type IN ('allow','deny')),
  PRIMARY KEY (channel_id, user_id)
);

CREATE TABLE IF NOT EXISTS config (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
-- runtime rows set via /configure:
--   ('watch_channel_id',  '<snowflake>')
--   ('party_category_id', '<snowflake>')
