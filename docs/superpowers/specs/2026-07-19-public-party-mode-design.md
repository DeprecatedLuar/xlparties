# Public party mode - design

## Goal

Add a fourth `/party_mode` value, `public`: anyone can see and join the
channel except users the owner has globally blocked (`/enemy_add`), or
explicitly denied for this channel (`/party_ban`, `/party_block`). Same
selection UX as the existing three modes (slash choice or button).

## Overwrite model

Every existing mode is default-deny: `@everyone: deny` plus explicit
per-member `allow` overwrites (friends, friends-of-friends sources, pending
`/party_invite` grants), with `party_overrides` (manual `/party_allow`,
`/party_block`, `/party_ban`) layered last so it always wins.

`public` flips the default to allow: `@everyone: allow`, with member-level
`deny` overwrites only for the owner's globally-blocked users
(`store.BlockIDs(ownerID)`) - the mirror image of how friends are the
auto-allow set in `friends_only`. `party_overrides` still applies last and
still wins over the auto-deny, so an owner can `/party_allow` a
globally-blocked person into this one channel if they choose, exactly like
today's precedence for every other mode.

Friend list, friends-of-friends sources, and pending invites are irrelevant
in `public` mode (default access already covers everyone) and are not
loaded/used when building its overwrite set - mirroring how `friends_only`
already skips crawling FoF sources and `invite_only` already skips the
friend list.

`buildRewriteOverwrites` gains a `mode string` and `blockedIDs []int64`
parameter. `rewriteOverwrites` (handoff.go) skips loading `friendIDs` when
mode is `invite_only` or `public` (extending the existing invite_only
special case), and loads `BlockIDs(ownerID)` only when mode is `public`.

`buildCreationOverwrites` is unaffected: new parties always start in
`friends_of_friends` (schema default), so a party only reaches `public` via
`/party_mode` after creation, which goes through `rewriteOverwrites`.

`SetAccessMode` (mode.go) adds `store.AccessModePublic` to its valid-mode
switch. No other change needed there: the existing "leaving/entering
friends_of_friends" branches already key off `!= AccessModeFriendsOfFriends`,
which correctly treats `public` like any other non-FoF mode.

## `/party_invite` interaction

`InviteToParty` inspects the target's current channel overwrite first: a
standing `deny` overwrite is refused outright. Since a globally-blocked user
in `public` mode already carries an explicit `deny` member overwrite (built
above), that path already refuses them correctly with no code change.

For a non-blocked target in `public` mode, no member overwrite exists (they
rely on the `@everyone: allow` default), so without a fix `InviteToParty`
would fall through to granting a redundant temp-allow overwrite, a
`party_invites` row, and an expiry timer - harmless (self-corrects on
expiry) but wasteful. Add an explicit check: if the party's mode is
`public` and the target has no `deny` overwrite, treat the invite as
`InviteAlreadyHasAccess` (DM only, no grant), instead of falling through to
the grant path.

## Schema

Widen the `access_mode` CHECK constraint to include `'public'`:

- `schema.sql`: add `'public'` to the CHECK list (fresh DBs get it
  directly).
- `migrate.go`: extend `migratePartiesAccessModeCheck` (rename if it reads
  oddly) to detect a table predating `public` and rebuild it, following the
  exact rebuild-and-copy pattern already used to add `invite_only` (SQLite
  cannot `ALTER TABLE ... CHECK`). Update the `expectedColumns["parties"]`
  entry's DDL string to match.

## Surface-level updates

- `store.AccessModePublic = "public"` constant.
- `partyModeLabel["public"] = "Public"` (commands/party_mode.go) - shared by
  `/party_mode` and `/party_info`.
- Add to `partyModeButtonRow`'s mode list and to the `/party_mode` slash
  command's string choices (commands.go).
- `/help` text: mention public mode alongside the other three.
- Party-creation welcome message (`messages.PartyCreated`): mention public
  mode is available via `/party_mode`.

## Testing

- `overwrites_test.go`: new case for `public` mode - a blocked user gets a
  `deny` overwrite, a non-blocked/non-override user has no member overwrite
  and relies on `@everyone: allow`, and a manual `/party_allow` override on a
  blocked user still wins.
- `migrate_test.go`: a DB with a pre-`public` CHECK gets it widened,
  mirroring the existing `invite_only` migration test.
- `go build ./...`, `gofmt -w .`, `go vet ./...`, `go test ./...` all clean.
- Live smoke test (optional, manual): switch a party to `public`, confirm a
  non-friend can join without any grant; confirm a globally-blocked user
  cannot join; confirm `/party_ban` still works on top of `public`.

## Out of scope

- No change to friends-of-friends scanning, handoff, or cleanup timers -
  `public` is just another non-FoF mode to them, same as `friends_only` and
  `invite_only` today.
- No new command; `/party_mode public` is the only entry point.
