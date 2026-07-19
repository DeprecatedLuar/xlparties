# Random greetings + party-creation opener

## Problem

Three DM notices in `internal/messages/messages.go` hardcode the same opener,
`"## Salutations Hooman.\n"`:

- `FriendAddedNotif`
- `PartyInviteDMAlreadyHasAccess`
- `PartyInviteDMBody`

Every friend-add / invite notification reads identically. We want variety
without hand-writing dozens of full alternate messages.

Separately, `PartyCreated`'s opener (`"## Salutations <@%d>.\n"`) is being
replaced with a fixed, non-random Subnautica-flavored welcome line — this is
a one-off wording change, not part of the random pool.

## Design

### Greeting pool (`internal/messages/greetings.go`, new file)

Two word pools, combined at random into `"<Opener>, <Subject>."`:

```go
var openers = []string{
	"Yo", "Psst", "Oi", "Behold", "Attention", "Heads up", "Look alive",
	"Watch out", "Howdy", "Salutations", "Greetings", "Hark", "Listen up",
	"Fear not", "BEWARE", "Giddy up", "Fasten your seatbelts",
}

var subjects = []string{
	"Hooman", "buddy", "Good Fellow", "Mortal", "Traveler", "Partner",
	"Meatbag", "Matey",
}
```

`var`, not `const` — Go does not allow slice-typed constants.

```go
func RandomGreeting() string {
	return fmt.Sprintf("%s, %s.", openers[rand.Intn(len(openers))], subjects[rand.Intn(len(subjects))])
}
```

Uses `math/rand`'s package-level `Intn` (auto-seeded since Go 1.20, no manual
seeding needed). 17 × 8 = 136 combinations. Some pairings will read a little
odd grammatically (e.g. "Fasten your seatbelts, Mortal.") — accepted as part
of the tone, not a defect.

`RandomGreeting()` is an ordinary exported function returning a `string`. It
plugs into the existing `fmt.Sprintf` call sites as a normal `%s` argument,
the same way `<@%d>` supplies a user mention — no new type or template
syntax.

### Template changes (`internal/messages/messages.go`)

Each of the three templates drops its hardcoded opener line in favor of a
`%s` placeholder (greeting arg goes first, since it's the opening line):

```go
FriendAddedNotif              = " ## %s\nIt seems <@%%d> added you as a friend in **%%s**.\n..."
PartyInviteDMAlreadyHasAccess = "## %s\n<@%%d> tried to invite you..."
PartyInviteDMBody             = "## %s\n<@%%d> invited you to their party..."
```

(Escaping shown for clarity — actual edit just changes the literal opener
text to `%s`, all other verbs/format specifiers are untouched.)

`RandomGreeting()` already supplies its own trailing period, so the
placeholder line is `"## %s\n"` — no extra period after `%s`.

### Call-site changes (3 sites, each prepends `messages.RandomGreeting()` as the new first `Sprintf` arg)

- `internal/commands/friend_add.go:58`
- `internal/party/invite.go:105`
- `internal/party/invite.go:136`

### `PartyCreated` opener (hardcoded, not randomized)

Replace the current opener line only:

```
"## Welcome aboard, Captain <@%d>. All systems online.\n" + (rest of existing body, unchanged)
```

This is a one-off wording change — no pool, no `RandomGreeting()` call.

## Testing

`internal/messages/greetings_test.go`: pure-function sanity checks —
`RandomGreeting()` returns non-empty, matches the `"<opener>, <subject>."`
shape, and (looping N times) only ever returns combinations built from the
two known pools. No mocking needed; the function has no dependencies.

## Out of scope

- No broader sweep of other messages for greeting-worthy openers (explicitly
  scoped to the 3 known "Salutations Hooman" spots).
- No persistence/config for the pools — they're Go slices, edited in source
  if the pool ever needs to change.
