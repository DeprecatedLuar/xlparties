# SPEC.md — Party VC Bot (v1)

## Purpose

A Discord bot that manages private, per-user voice channels ("parties"). When a user joins a designated watch channel, the bot creates a private voice channel scoped to that user and their friends. The party owner can override access per channel. Access is controlled entirely through channel permission overwrites, not roles.

## Scope

This document covers v1 only. The following are explicitly deferred and out of scope:

- Any-member allow (v1 restricts allow/deny to the owner).
- Batch/multi-user command targets.
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

SQLite. Six tables.

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
  channel_id  INTEGER PRIMARY KEY,   -- the party voice channel snowflake
  owner_id    INTEGER NOT NULL,      -- current owner snowflake
  created_at  INTEGER NOT NULL,
  access_mode TEXT NOT NULL DEFAULT 'friends_of_friends' CHECK (access_mode IN ('friends_of_friends','friends_only','invite_only'))
);

CREATE TABLE party_overrides (
  channel_id INTEGER NOT NULL REFERENCES parties(channel_id),
  user_id    INTEGER NOT NULL,
  type       TEXT NOT NULL CHECK (type IN ('allow','deny')),
  PRIMARY KEY (channel_id, user_id)
);

CREATE TABLE party_sources (
  channel_id INTEGER NOT NULL REFERENCES parties(channel_id),
  user_id    INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
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

`parties` is the persistent record of which channels the bot created and who currently owns each. It is the source of truth for the startup sweep, ownership tracking, and the duplicate-ownership guard. It is not held in memory alone. A row is inserted at party creation and deleted at party cleanup, matching the channel lifecycle. `owner_id` is updated on ownership handoff. `access_mode` selects which access mode applies to the channel (see Access Modes); it defaults to `friends_of_friends` and every party is created in that mode. It is not derivable from anything else — it is manual, channel-local owner intent, set via `/party_mode` — and must be stored so it survives a bot restart, since the startup sweep and the friends-of-friends scan both branch on it.

### party_overrides

`party_overrides` stores per-channel manual access decisions set by `/party_allow` and `/party_deny`. It stores only manual overrides, not the creation-time friend whitelist. The friend whitelist is derived at build time from `relationships` and is not duplicated here.

Manual overrides are stored because they cannot be reconstructed from any other source. They are channel-local intent. Friend defaults are not stored because they are reconstructable from `relationships`. The rule is: store what cannot be derived, derive what can.

The `PRIMARY KEY (channel_id, user_id)` enforces one override per user per channel, mirroring Discord's single member-overwrite slot per user per channel. A later `/party_deny` on a user who has an existing `/party_allow` upserts the row from `allow` to `deny`, matching the overwrite behavior. Rows are deleted when the party is cleaned up.

### party_sources

`party_sources` stores which non-owner members currently count as active friends-of-friends scan sources for a channel (see Friends-of-Friends Growth). It is the one piece of growth state that cannot be derived from `relationships` alone, since membership in it is driven by voice presence and timers, not by a relationship edge. The resulting allow-set — the union of `FriendIDs` over every source row for the channel — is never stored; it is crawled fresh each time the channel's overwrites are rebuilt, matching the same "store what cannot be derived, derive what can" rule already applied to `party_overrides`.

Rows are inserted when a non-owner member has been present in the channel past the join delay, and deleted when a source has been absent past the leave grace (unless another still-present source also vouches for the same friend, in which case the union naturally still includes them). Rows are deleted in bulk when the party is cleaned up or switched away from `friends_of_friends` (to either `friends_only` or `invite_only`).

## Relationship Commands

`/friend_add @user`
Upsert `(caller, user, 'friend')` via `INSERT ... ON CONFLICT (granter_id, grantee_id) DO UPDATE SET relation_type = 'friend', created_at = <now>`. Upsert the `users` rows for both if absent. Upsert is required because a prior `block` edge for the same pair would otherwise violate the primary key — adding a friend therefore also clears any existing enemy status for that pair.

`/friend_remove @user`
Delete `(caller, user, 'friend')`.

`/friend_list`
List every user the caller has a `'friend'` edge to.

`/enemy_add @user`
Upsert `(caller, user, 'block')` via `INSERT ... ON CONFLICT (granter_id, grantee_id) DO UPDATE SET relation_type = 'block', created_at = <now>`. Upsert the `users` rows for both if absent. Same upsert reasoning as `/friend_add`, in reverse: adding an enemy also clears any existing friend status for that pair.

`/enemy_remove @user`
Delete `(caller, user, 'block')`.

`/enemy_list`
List every user the caller has a `'block'` edge to.

All six modify or read only the caller's own edges. Naming is noun-first (`friend_*` / `enemy_*`) for parity; `relation_type` in the schema keeps its original `'friend'`/`'block'` values — only the command-facing name is `enemy_*`.

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

Owner-only commands (`/party_allow`, `/party_deny`) act on behalf of whoever currently holds ownership.

Absence is measured from the owner's departure from the party channel. If the owner returns before 60 seconds elapse, no handoff occurs. The 60-second timer is in-memory. On bot restart the timer is not restored; ownership state is read from `parties`, and any pending handoff is re-evaluated against current channel membership.

### Ownership Rewrite

The rewrite sets the channel's access to the new owner's defaults, then applies the channel's manual overrides on top. It does not disconnect anyone. Removing a member's allow overwrite does not eject a member who is already connected; the denial applies only to future connection attempts. Members already in the channel remain until they leave on their own.

Desired overwrite state after handoff:
1. `@everyone`: deny `VIEW_CHANNEL`, deny `CONNECT`.
2. The new owner: allow `VIEW_CHANNEL`, allow `CONNECT`.
3. Each of the new owner's friends (from `relationships`): allow `VIEW_CHANNEL`, allow `CONNECT`. Skipped entirely in `invite_only` mode (see Access Modes) — the new owner's friend list confers no default access there, same as it wouldn't have before the handoff.
4. Each friend of each row in `party_sources` for this channel (see Friends-of-Friends Growth): allow `VIEW_CHANNEL`, allow `CONNECT`. `party_sources` is always empty outside `friends_of_friends` mode, so this step is a no-op in `friends_only`/`invite_only`.
5. Each row in `party_overrides` for this channel, applied last: `allow` rows allow `VIEW_CHANNEL`/`CONNECT`; `deny` rows deny `VIEW_CHANNEL`/`CONNECT`.

The desired state is computed in memory and applied as a bulk channel edit. Because `party_overrides` is applied last, a manual `/party_deny` still overrides a new-owner friend default or a friends-of-friends grant, and a manual `/party_allow` still grants a non-friend, consistent with Access Resolution. `party_sources` rows are not reset on handoff — sources belong to the channel, not the owner, so growth continues uninterrupted across a handoff.

### Access Modes

Every party is created in `friends_of_friends` mode (the `parties.access_mode` default). The owner can switch modes at any point via `/party_mode` (see Per-Channel Access Commands); the mode is stored on the `parties` row and survives a restart.

- **`friends_of_friends`** — the owner's friends have default access, and any non-owner member who stays connected long enough becomes a scan source whose own friends also gain access. See Friends-of-Friends Growth.
- **`friends_only`** — only the owner's friends have default access. No scan sources are tracked; any active `party_sources` rows are dropped when switching into this mode.
- **`invite_only`** — the owner's friend list confers no default access at all. Only the owner and users with an explicit `/party_allow` override (see Per-Channel Access Commands) can see or join the channel. Like `friends_only`, no scan sources are tracked.

Switching *out of* `friends_of_friends` drops all current `party_sources` rows for the channel and cancels their pending join/leave timers — their contribution stops applying immediately, per the Ownership Rewrite/Access Resolution rules for the new mode. Switching *into* `friends_of_friends` arms a fresh join-maturation timer for every non-owner member currently in the channel, so growth resumes without requiring a leave/rejoin. In every case `party_overrides` rows are untouched by a mode switch — manual `/party_allow`/`/party_deny` decisions persist across mode changes.

### Friends-of-Friends Growth

In `friends_of_friends` mode, any non-owner member present in the party channel for a while becomes an active scan source, and that source's own friends gain access to the channel — access grows transitively, not just from the owner's friend list. Access is crawled live from every current source rather than materialized into a stored allow-list, so it naturally stacks as more members pass through, and self-corrects if a source's friend list changes before the next rebuild.

Mechanic:
1. **Join scan.** When a non-owner member joins a `friends_of_friends` party and remains connected for 5 seconds (`friendOfFriendJoinDelay`), they become an active source: a `party_sources` row is inserted for `(channel_id, user_id)`, and the channel's overwrites are rebuilt to include that source's friends (see Ownership Rewrite step 4). If the member leaves before the delay elapses, no row is ever inserted.
2. **Leave revoke.** When an active source leaves the channel and remains absent for 15 seconds (`friendOfFriendLeaveGrace`), their `party_sources` row is deleted and the overwrites are rebuilt. Because the allow-set is a live union across all current sources, a friend who is also vouched for by another still-present source keeps their access — only the departing source's own unique contribution is lost. If the member rejoins before the grace period elapses, the pending removal is cancelled and nothing changes.
3. **Deny still wins.** A `/party_deny` override on a would-be grantee is applied last, exactly as in Ownership Rewrite, so it continues to block them even after a scan fires in their favor.
4. **Unfriending drops access on the next rebuild.** Because the allow-set is crawled live rather than stored, a source unfriending someone removes that person from the allow-set the next time the channel's overwrites are rebuilt (handoff, another scan event, or a manual override) — no explicit revocation step is needed.

The join/leave timers are in-memory, keyed per `(channel_id, user_id)`, and are not restored across a bot restart. The startup sweep reconciles this for each `friends_of_friends` party against current channel membership: any `party_sources` row whose user is no longer present is pruned (their leave timer never got to fire, and without this fix they would keep granting access indefinitely) and the channel's overwrites are rebuilt if any were pruned; any present non-owner member who is not already a source gets a fresh join-maturation timer (any partial progress toward the 5-second delay from before the restart is lost, the same accepted cost already applied to the handoff and cleanup timers). `party_sources` rows for members who stayed connected across the restart are left untouched and keep contributing without interruption.

### Cleanup

A party channel is deleted when it has been empty for the configured grace period (`EMPTY_CLEANUP_SECONDS`, default 30). The grace period exists to absorb Discord voice-state flicker (a member's connection briefly dropping and reconnecting), not to keep idle channels alive; it is deliberately short.

The grace timer is in-memory. When a channel becomes empty, a timer starts. If a member joins before it fires, the timer is cancelled. If the channel is still empty when the timer fires, the channel is deleted.

On bot restart the grace timer is not restored. A restart mid-grace-period restarts the grace period for any channel still empty. The cost of an occasionally over-long or over-short grace period after a restart is accepted.

Cleanup does not rely on the timer alone. On bot startup, the bot sweeps all rows in `parties`:
- If the channel no longer exists on Discord, delete the `parties` row and its `party_overrides` rows.
- If the channel exists and is empty, apply the normal grace period.
- If the channel exists and is non-empty, resume normal operation, re-evaluate ownership against current membership, and reconcile friends-of-friends scan sources against current membership (see Friends-of-Friends Growth).

Deleting the party channel deletes all its overwrites, clearing that portion of the guild overwrite budget. The `parties` row and its `party_overrides` rows are deleted at the same time.

## Per-Channel Access Commands

Both commands are owner-only. Both act on the party channel the caller is in. If the command is run outside an active party channel, or by a non-owner, it is refused with an ephemeral error.

`/party_allow @user`
Set overwrite on `@user` for the current party channel: allow `VIEW_CHANNEL`, allow `CONNECT`. Upsert `(channel_id, user.id, 'allow')` into `party_overrides`. Overrides all defaults, including a block. Does not modify the relationship graph. Reply is public.

`/party_deny @user`
Set overwrite on `@user` for the current party channel: deny `VIEW_CHANNEL`, deny `CONNECT`. Upsert `(channel_id, user.id, 'deny')` into `party_overrides`. Overrides all defaults, including the creation-time friend whitelist. Does not modify the relationship graph. Reply is ephemeral.

`/party_deny` does not disconnect a user who is currently connected. The deny applies to future connection attempts only; a connected user remains until they leave on their own, after which they cannot rejoin.

Both take a single user. Multi-user targets are out of scope for v1.

`/party_mode [mode]`
Owner-only, acts on the party channel the caller is in, same refusal rules as `/party_allow`/`/party_deny`. `mode` is an optional choice of `friends_of_friends` / `friends_only` / `invite_only` (see Access Modes):
- If `mode` is given, the channel's `access_mode` is updated and its overwrites are rewritten immediately to match the new mode. Reply is public.
- If `mode` is omitted, the bot replies ephemerally with three buttons, one per mode; clicking one applies that mode the same way the slash-command argument would, and edits the ephemeral message in place to confirm.

## Access Resolution

For a given party channel, effective access to a user is:

1. If a `/party_deny` override exists for the user, they are denied. (Deny override wins.)
2. Else if a `/party_allow` override exists for the user, they are allowed.
3. Else if the channel is in `friends_of_friends` or `friends_only` mode and the user has a friend overwrite — either a direct friend of the owner (creation-time, or rewritten at handoff), or (in `friends_of_friends` mode only) a friend of an active scan source (see Friends-of-Friends Growth) — they are allowed. In `invite_only` mode this step is skipped entirely; the owner's friend list confers no access.
4. Else they are denied by the `@everyone` deny.

This ordering is enforced by Discord's native overwrite model. There is one member-overwrite slot per user per channel; `/party_allow` and `/party_deny` write to the same slot, so the most recent owner action wins. No custom resolution logic is required beyond writing the correct overwrite.

Per-channel overrides are local to the channel. They are stored in `party_overrides` for the channel's lifetime and are discarded when the channel is deleted.

## Overwrite Budget

Discord enforces a limit of 1000 permission overwrites per guild, summed across all channels. Each member or role entry on a channel is one overwrite; allows and denies count equally. A party channel consumes one overwrite for `@everyone`, one for the owner, one per person in the current allow-set (owner's direct friends plus every friend-of-a-source under Friends-of-Friends Growth), and one per manual override.

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

If `watch_channel_id` is unset at bot startup, log the condition and disable party creation (do not silently watch nothing). Other commands (`/friend_add`, `/enemy_add`, etc.) remain functional.

## Command Summary

| Command | Actor | Scope | Effect |
|---|---|---|---|
| `/friend_add @user` | any | global | upsert friend edge |
| `/friend_remove @user` | any | global | delete friend edge |
| `/friend_list` | any | global | list caller's friend edges |
| `/enemy_add @user` | any | global | upsert block edge |
| `/enemy_remove @user` | any | global | delete block edge |
| `/enemy_list` | any | global | list caller's block edges |
| `/party_allow @user` | party owner | current channel | allow overwrite + `party_overrides` row, overrides block |
| `/party_deny @user` | party owner | current channel | deny overwrite + `party_overrides` row, overrides friend |
| `/party_mode [mode]` | party owner | current channel | set `access_mode`, rewrite overwrites; omitted mode prompts buttons |
| `/configure watch_channel #channel` | Manage Guild | guild | set the party-trigger channel |
| `/configure category <category>` | Manage Guild | guild | set the category new party channels spawn under |
| `/help` | any | ephemeral | list available commands |

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
