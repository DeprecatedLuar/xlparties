# SPEC.md — Party VC Bot (v1)

## Purpose

A Discord bot that manages private, per-user voice channels ("parties"). When a user joins a designated watch channel, the bot creates a private voice channel scoped to that user and their friends. The party owner can override access per channel. Access is controlled entirely through channel permission overwrites, not roles.

## Scope

This document covers v1 only. The following are explicitly deferred and out of scope:

- Friends-of-friends transitive access.
- Join/leave rescan logic for access recomputation, except the ownership-handoff rewrite defined below.
- Any-member allow (v1 restricts allow/deny to the owner).
- Batch/multi-user command targets.
- Per-channel access-mode switching (friends / friends-of-friends / invite-only).
- DM notifications.
- OAuth2 `relationships.read` integration with Discord's real friend graph.
- Overwrite-budget pre-check at party creation (see Overwrite Budget).

## Constraints

- The bot runs in a single guild. Cross-guild behavior is undefined and not handled.
- The friend graph is internal to the bot. Discord's native friend list is not readable by bots without gated OAuth2 scopes and is not used.
- Access is enforced via member-specific channel permission overwrites. Roles are not used for party access.
- Discord enforces a limit of 1000 permission overwrites per guild, summed across all channels (see Overwrite Budget). Party channels and their overwrites are deleted on cleanup, which clears the budget. Cleanup must be resilient (see Cleanup).
- Slash commands only. The bot does not read message content and does not request the `MESSAGE_CONTENT` privileged intent.

## Data Model

SQLite. Five tables.

```sql
CREATE TABLE users (
  id INTEGER PRIMARY KEY   -- Discord user snowflake
);

CREATE TABLE relationships (
  granter_id    INTEGER NOT NULL REFERENCES users(id),
  grantee_id    INTEGER NOT NULL REFERENCES users(id),
  relation_type TEXT NOT NULL CHECK (relation_type IN ('friend','block')),
  created_at    INTEGER NOT NULL,
  PRIMARY KEY (granter_id, grantee_id)
);

CREATE INDEX idx_grantee ON relationships(grantee_id);

CREATE TABLE parties (
  channel_id INTEGER PRIMARY KEY,   -- the party voice channel snowflake
  owner_id   INTEGER NOT NULL,      -- current owner snowflake
  created_at INTEGER NOT NULL
);

CREATE TABLE party_overrides (
  channel_id INTEGER NOT NULL REFERENCES parties(channel_id),
  user_id    INTEGER NOT NULL,
  type       TEXT NOT NULL CHECK (type IN ('allow','deny')),
  PRIMARY KEY (channel_id, user_id)
);

CREATE TABLE config (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
-- runtime rows set via /configure:
--   ('watch_channel_id',  '<snowflake>')
--   ('party_category_id', '<snowflake>')
```

Each relationship is one directed edge, one row. Friend lists are computed by query, not stored as a list in a column.

A directed edge `(granter_id, grantee_id, 'friend')` means: `granter_id` allows `grantee_id` to see and join `granter_id`'s party by default.

A user is either a friend or a block relative to a given granter, not both. The `PRIMARY KEY (granter_id, grantee_id)` enforces one relationship per pair; changing type is handled by upsert (see Relationship Commands).

Relationship edges are global to the user within the guild. They are not scoped per channel.

### parties

`parties` is the persistent record of which channels the bot created and who currently owns each. It is the source of truth for the startup sweep, ownership tracking, and the duplicate-ownership guard. It is not held in memory alone. A row is inserted at party creation and deleted at party cleanup, matching the channel lifecycle. `owner_id` is updated on ownership handoff.

### party_overrides

`party_overrides` stores per-channel manual access decisions set by `/vc_allow` and `/vc_deny`. It stores only manual overrides, not the creation-time friend whitelist. The friend whitelist is derived at build time from `relationships` and is not duplicated here.

Manual overrides are stored because they cannot be reconstructed from any other source. They are channel-local intent. Friend defaults are not stored because they are reconstructable from `relationships`. The rule is: store what cannot be derived, derive what can.

The `PRIMARY KEY (channel_id, user_id)` enforces one override per user per channel, mirroring Discord's single member-overwrite slot per user per channel. A later `/vc_deny` on a user who has an existing `/vc_allow` upserts the row from `allow` to `deny`, matching the overwrite behavior. Rows are deleted when the party is cleaned up.

## Relationship Commands

`/friend_add @user`
Upsert `(caller, user, 'friend')` via `INSERT ... ON CONFLICT (granter_id, grantee_id) DO UPDATE SET relation_type = 'friend', created_at = <now>`. Upsert the `users` rows for both if absent. Upsert is required because a prior `block` edge for the same pair would otherwise violate the primary key.

`/friend_remove @user`
Delete `(caller, user, 'friend')`.

`/block @user`
Upsert `(caller, user, 'block')` via `INSERT ... ON CONFLICT (granter_id, grantee_id) DO UPDATE SET relation_type = 'block', created_at = <now>`. Upsert the `users` rows for both if absent.

`/unblock @user`
Delete `(caller, user, 'block')`.

All four modify only the caller's own edges. Naming uses noun-first grouping for `friend_*`; `block` / `unblock` retained as idiomatic pair.

## Party Lifecycle

### Watch channel

One voice channel in the guild is designated the watch channel, set via `/configure watch_channel #channel` (see Configuration). The bot listens for voice-state updates and compares the target channel id against the stored value:

```
on voice_state_update(user, channel):
  if channel.id == config.get('watch_channel_id'):
    spawn_party(user)
```

### Creation

Trigger: a user joins the watch channel.

Before creating, the bot checks `parties` for an existing row with `owner_id = user.id`. If the user already owns an active party, creation is skipped (no duplicate party per owner). The user is not moved.

If no active party exists for the user, the bot builds the full overwrite set in memory, then creates the channel in a single API call with the overwrites attached. Discord's Create Guild Channel endpoint accepts a `permission_overwrites` array in the request body, so the entire set is applied atomically at creation. There is no sequential post-creation loop of overwrite writes.

The overwrite set at creation:
1. `@everyone`: deny `VIEW_CHANNEL`, deny `CONNECT`.
2. The owner: allow `VIEW_CHANNEL`, allow `CONNECT`.
3. Each of the owner's friends (`SELECT grantee_id FROM relationships WHERE granter_id = :owner_id AND relation_type = 'friend'`): allow `VIEW_CHANNEL`, allow `CONNECT`.

After the channel is created, the bot inserts the `parties` row (`channel_id`, `owner_id`, `created_at`) and moves the owner into the party channel.

Blocked users receive no overwrite and are excluded by default. No explicit block handling is needed at creation because a blocked user is not in the friend set.

Applying all overwrites in the single creation call removes the partial-state window in which the channel exists but some friend overwrites have not yet been written, and removes the sequential-write burst that would otherwise occur under concurrent joins.

### Ownership

The owner is the user for whom the party channel was created. Ownership is recorded in the `parties` row for the channel.

Ownership handoff is triggered when the current owner is absent from the party channel for longer than 60 seconds while at least one other member remains connected. On trigger:

1. Select a new owner at random from the members currently connected to the party channel. No join-order tracking is used; the set of current members is read from the library's voice-state cache.
2. Update `parties.owner_id` to the new owner.
3. Rewrite the channel overwrites to the new owner's whitelist (see Ownership Rewrite).
4. Post a message in the party channel's text chat notifying the new owner that they now hold ownership.

Owner-only commands (`/vc_allow`, `/vc_deny`) act on behalf of whoever currently holds ownership.

Absence is measured from the owner's departure from the party channel. If the owner returns before 60 seconds elapse, no handoff occurs. The 60-second timer is in-memory. On bot restart the timer is not restored; ownership state is read from `parties`, and any pending handoff is re-evaluated against current channel membership.

### Ownership Rewrite

The rewrite sets the channel's access to the new owner's defaults, then applies the channel's manual overrides on top. It does not disconnect anyone. Removing a member's allow overwrite does not eject a member who is already connected; the denial applies only to future connection attempts. Members already in the channel remain until they leave on their own.

Desired overwrite state after handoff:
1. `@everyone`: deny `VIEW_CHANNEL`, deny `CONNECT`.
2. The new owner: allow `VIEW_CHANNEL`, allow `CONNECT`.
3. Each of the new owner's friends (from `relationships`): allow `VIEW_CHANNEL`, allow `CONNECT`.
4. Each row in `party_overrides` for this channel, applied last: `allow` rows allow `VIEW_CHANNEL`/`CONNECT`; `deny` rows deny `VIEW_CHANNEL`/`CONNECT`.

The desired state is computed in memory and applied as a bulk channel edit. Because `party_overrides` is applied last, a manual `/vc_deny` still overrides a new-owner friend default and a manual `/vc_allow` still grants a non-friend, consistent with Access Resolution.

### Cleanup

A party channel is deleted when it has been empty for the configured grace period (`EMPTY_CLEANUP_SECONDS`, default 30). The grace period exists to absorb Discord voice-state flicker (a member's connection briefly dropping and reconnecting), not to keep idle channels alive; it is deliberately short.

The grace timer is in-memory. When a channel becomes empty, a timer starts. If a member joins before it fires, the timer is cancelled. If the channel is still empty when the timer fires, the channel is deleted.

On bot restart the grace timer is not restored. A restart mid-grace-period restarts the grace period for any channel still empty. The cost of an occasionally over-long or over-short grace period after a restart is accepted.

Cleanup does not rely on the timer alone. On bot startup, the bot sweeps all rows in `parties`:
- If the channel no longer exists on Discord, delete the `parties` row and its `party_overrides` rows.
- If the channel exists and is empty, apply the normal grace period.
- If the channel exists and is non-empty, resume normal operation and re-evaluate ownership against current membership.

Deleting the party channel deletes all its overwrites, clearing that portion of the guild overwrite budget. The `parties` row and its `party_overrides` rows are deleted at the same time.

## Per-Channel Access Commands

Both commands are owner-only. Both act on the party channel the caller is in. If the command is run outside an active party channel, or by a non-owner, it is refused with an ephemeral error.

`/vc_allow @user`
Set overwrite on `@user` for the current party channel: allow `VIEW_CHANNEL`, allow `CONNECT`. Upsert `(channel_id, user.id, 'allow')` into `party_overrides`. Overrides all defaults, including a block. Does not modify the relationship graph. Reply is public.

`/vc_deny @user`
Set overwrite on `@user` for the current party channel: deny `VIEW_CHANNEL`, deny `CONNECT`. Upsert `(channel_id, user.id, 'deny')` into `party_overrides`. Overrides all defaults, including the creation-time friend whitelist. Does not modify the relationship graph. Reply is ephemeral.

`/vc_deny` does not disconnect a user who is currently connected. The deny applies to future connection attempts only; a connected user remains until they leave on their own, after which they cannot rejoin.

Both take a single user. Multi-user targets are out of scope for v1.

## Access Resolution

For a given party channel, effective access to a user is:

1. If a `/vc_deny` override exists for the user, they are denied. (Deny override wins.)
2. Else if a `/vc_allow` override exists for the user, they are allowed.
3. Else if the user has a friend overwrite (creation-time, or rewritten at handoff), they are allowed.
4. Else they are denied by the `@everyone` deny.

This ordering is enforced by Discord's native overwrite model. There is one member-overwrite slot per user per channel; `/vc_allow` and `/vc_deny` write to the same slot, so the most recent owner action wins. No custom resolution logic is required beyond writing the correct overwrite.

Per-channel overrides are local to the channel. They are stored in `party_overrides` for the channel's lifetime and are discarded when the channel is deleted.

## Overwrite Budget

Discord enforces a limit of 1000 permission overwrites per guild, summed across all channels. Each member or role entry on a channel is one overwrite; allows and denies count equally. A party channel consumes one overwrite for `@everyone`, one for the owner, one per friend, and one per manual override.

For v1, no pre-check is performed at creation. At MVP scale the guild is not expected to approach the cap. If the Create Guild Channel call fails due to the cap, the failure is logged and party creation for that user fails for that attempt. A pre-check that lists guild channels, sums their overwrites, and refuses creation when the projected total would exceed the cap is deferred to a later version.

## Permissions Required

Bot requires guild permissions: `Manage Channels`, `Manage Roles`, `Move Members`, `View Channels`, `Connect`.

`Manage Roles` is required to set member permission overwrites on channels the bot creates.

The bot's highest role must sit above the roles of users it moves or disconnects, per Discord's role-hierarchy rule for member actions.

## Configuration

`/configure watch_channel #channel`
Upsert `('watch_channel_id', channel.id)` in `config`. Only one watch channel is tracked; re-running replaces the prior value. Requires `Manage Guild` permission (native Discord check, not a custom role).

`/configure category <category>`
Upsert `('party_category_id', category.id)` in `config`. Sets the category new party channels are created under. Only one category is tracked; re-running replaces the prior value. If unset, party channels are created at the guild root. Requires `Manage Guild` permission.

Re-configuring affects future parties only. Party channels already spawned are independent of the watch channel at that point and are unaffected. They run out their normal lifecycle (ownership handoff, empty-cleanup) regardless of what the watch channel is later set to.

If `watch_channel_id` is unset at bot startup, log the condition and disable party creation (do not silently watch nothing). Other commands (`/friend_add`, `/block`, etc.) remain functional.

## Command Summary

| Command | Actor | Scope | Effect |
|---|---|---|---|
| `/friend_add @user` | any | global | upsert friend edge |
| `/friend_remove @user` | any | global | delete friend edge |
| `/block @user` | any | global | upsert block edge |
| `/unblock @user` | any | global | delete block edge |
| `/vc_allow @user` | party owner | current channel | allow overwrite + `party_overrides` row, overrides block |
| `/vc_deny @user` | party owner | current channel | deny overwrite + `party_overrides` row, overrides friend |
| `/configure watch_channel #channel` | Manage Guild | guild | set the party-trigger channel |
| `/configure category <category>` | Manage Guild | guild | set the category new party channels spawn under |

## Configuration Values

Two homes. Deploy-time constants live in `.env` (loaded at startup). Runtime channel/category targets live in the `config` table, set via `/configure`, and persist across restarts.

`.env` (deploy-time):
- `DISCORD_TOKEN`, `DISCORD_APP_ID`, `DISCORD_PUBLIC_KEY` — bot credentials.
- `DB_PATH` — SQLite file path. Required; startup hard-fails if unset (no silent default). Use an absolute path when running as a service.
- `EMPTY_CLEANUP_SECONDS` — empty-channel grace period. Default 30.
- `OWNER_ABSENCE_HANDOFF_SECONDS` — owner-absence handoff threshold. Default 60.

`config` table (runtime, via `/configure`):
- `watch_channel_id` — the party-trigger channel. Unset disables party creation (see Configuration).
- `party_category_id` — the category new party channels spawn under. Unset creates them at the guild root.

The guild id is not configured; the bot is single-guild and derives it from the gateway `READY` event at startup.

## Verification Notes

Two platform behaviors are relied on and should be confirmed by manual test before the ownership-rewrite path is treated as load-bearing:

- Removing a member's allow overwrite does not disconnect a member already connected to the voice channel. The denial applies to future connection attempts only.
- The 1000-overwrite limit is a guild-wide total summed across all channels, not a per-channel limit.
