# Random Greetings Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the hardcoded "## Salutations Hooman." opener in three DM notices with a randomly-generated greeting, and give the party-creation channel message a fixed Subnautica-themed opener.

**Architecture:** A new `internal/messages/greetings.go` file holds two word pools (`openers`, `subjects`) and an exported `RandomGreeting() string` that combines one of each into `"<Opener>, <Subject>."`. Three existing message templates in `internal/messages/messages.go` swap their literal opener text for a `%s` placeholder; their three call sites pass `messages.RandomGreeting()` as the new first `Sprintf` argument. `PartyCreated`'s opener is separately hardcoded to a fixed Subnautica line — no pool, no randomness.

**Tech Stack:** Go, standard library only (`math/rand`, `fmt`).

## Global Constraints

- No magic values: the `openers`/`subjects` word lists are declared at the top of `greetings.go`, the only file that uses them.
- `var`, not `const`, for the two slices — Go does not allow slice-typed constants.
- `RandomGreeting()`'s output already ends in a period; templates must not add a second one.
- `gofmt -w .` must be run before each commit (project convention).
- No new dependencies — `math/rand` needs no manual seeding (auto-seeded since Go 1.20).

---

### Task 1: Greeting pool + `RandomGreeting()`

**Files:**
- Create: `internal/messages/greetings.go`
- Test: `internal/messages/greetings_test.go`

**Interfaces:**
- Produces: `func RandomGreeting() string` — exported, no params, returns a string of the shape `"<Opener>, <Subject>."` where `<Opener>` is one of the 17 `openers` entries and `<Subject>` is one of the 8 `subjects` entries. Later tasks (Task 2) call this as a plain `%s` `Sprintf` argument.

- [ ] **Step 1: Write the failing test**

```go
// internal/messages/greetings_test.go
package messages

import (
	"strings"
	"testing"
)

func TestRandomGreeting(t *testing.T) {
	seenOpener := map[string]bool{}
	seenSubject := map[string]bool{}

	for i := 0; i < 200; i++ {
		greeting := RandomGreeting()

		if greeting == "" {
			t.Fatal("RandomGreeting returned empty string")
		}
		if !strings.HasSuffix(greeting, ".") {
			t.Fatalf("RandomGreeting() = %q, want suffix '.'", greeting)
		}
		if strings.HasSuffix(greeting, "..") {
			t.Fatalf("RandomGreeting() = %q, has double period", greeting)
		}

		parts := strings.SplitN(strings.TrimSuffix(greeting, "."), ", ", 2)
		if len(parts) != 2 {
			t.Fatalf("RandomGreeting() = %q, want shape '<opener>, <subject>.'", greeting)
		}

		opener, subject := parts[0], parts[1]
		if !containsString(openers, opener) {
			t.Fatalf("RandomGreeting() opener %q not in openers pool", opener)
		}
		if !containsString(subjects, subject) {
			t.Fatalf("RandomGreeting() subject %q not in subjects pool", subject)
		}

		seenOpener[opener] = true
		seenSubject[subject] = true
	}

	if len(seenOpener) < 2 {
		t.Errorf("expected multiple distinct openers across 200 calls, got %d", len(seenOpener))
	}
	if len(seenSubject) < 2 {
		t.Errorf("expected multiple distinct subjects across 200 calls, got %d", len(seenSubject))
	}
}

func containsString(pool []string, s string) bool {
	for _, p := range pool {
		if p == s {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/messages/... -run TestRandomGreeting -v`
Expected: FAIL (build error — `RandomGreeting`, `openers`, `subjects` undefined)

- [ ] **Step 3: Write minimal implementation**

```go
// internal/messages/greetings.go

// Package-level greeting pools used by RandomGreeting to vary the opener
// line on DM notices, so repeated notifications don't read identically.
package messages

import (
	"fmt"
	"math/rand"
)

var openers = []string{
	"Yo", "Psst", "Oi", "Behold", "Attention", "Heads up", "Look alive",
	"Watch out", "Howdy", "Salutations", "Greetings", "Hark", "Listen up",
	"Fear not", "BEWARE", "Giddy up", "Fasten your seatbelts",
}

var subjects = []string{
	"Hooman", "buddy", "Good Fellow", "Mortal", "Traveler", "Partner",
	"Meatbag", "Matey",
}

// RandomGreeting returns a randomly combined "<Opener>, <Subject>." line for
// use as the opener of a DM notice.
func RandomGreeting() string {
	opener := openers[rand.Intn(len(openers))]
	subject := subjects[rand.Intn(len(subjects))]
	return fmt.Sprintf("%s, %s.", opener, subject)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/messages/... -run TestRandomGreeting -v`
Expected: PASS

- [ ] **Step 5: Format and commit**

```bash
gofmt -w internal/messages/greetings.go internal/messages/greetings_test.go
git add internal/messages/greetings.go internal/messages/greetings_test.go
git commit -m "Add RandomGreeting for varied DM notice openers"
```

---

### Task 2: Wire `RandomGreeting()` into the three DM templates

**Files:**
- Modify: `internal/messages/messages.go:59` (`FriendAddedNotif`)
- Modify: `internal/messages/messages.go:103-104` (`PartyInviteDMAlreadyHasAccess`, `PartyInviteDMBody`)
- Modify: `internal/commands/friend_add.go:58`
- Modify: `internal/party/invite.go:105`
- Modify: `internal/party/invite.go:136`

**Interfaces:**
- Consumes: `messages.RandomGreeting() string` from Task 1.

- [ ] **Step 1: Update the three templates in `internal/messages/messages.go`**

Change (line 59):
```go
	FriendAddedNotif = " ## Salutations Hooman.\nIt seems <@%d> added you as a friend in **%s**.\nI would never do that _but_ you can use `/friend_add` (in the server) and pick <@%d> as the user to add them back"
```
to:
```go
	FriendAddedNotif = " ## %s\nIt seems <@%d> added you as a friend in **%s**.\nI would never do that _but_ you can use `/friend_add` (in the server) and pick <@%d> as the user to add them back"
```

Change (lines 103-104):
```go
	PartyInviteDMAlreadyHasAccess = "## Salutations Hooman.\n<@%d> tried to invite you to their party in this server, but you already have access - hop into their party voice channel whenever you like."
	PartyInviteDMBody             = "## Salutations Hooman.\n<@%d> invited you to their party in this server. Here's your one-click join link (works once, and only until you use it or it expires):\n%s"
```
to:
```go
	PartyInviteDMAlreadyHasAccess = "## %s\n<@%d> tried to invite you to their party in this server, but you already have access - hop into their party voice channel whenever you like."
	PartyInviteDMBody             = "## %s\n<@%d> invited you to their party in this server. Here's your one-click join link (works once, and only until you use it or it expires):\n%s"
```

- [ ] **Step 2: Update the three call sites**

`internal/commands/friend_add.go:58`, change:
```go
	msg := fmt.Sprintf(messages.FriendAddedNotif, caller, guildName, caller)
```
to:
```go
	msg := fmt.Sprintf(messages.FriendAddedNotif, messages.RandomGreeting(), caller, guildName, caller)
```

`internal/party/invite.go:105`, change:
```go
	msg := fmt.Sprintf(messages.PartyInviteDMAlreadyHasAccess, callerID)
```
to:
```go
	msg := fmt.Sprintf(messages.PartyInviteDMAlreadyHasAccess, messages.RandomGreeting(), callerID)
```

`internal/party/invite.go:136`, change:
```go
	msg := fmt.Sprintf(messages.PartyInviteDMBody, callerID, "https://discord.gg/"+invite.Code)
```
to:
```go
	msg := fmt.Sprintf(messages.PartyInviteDMBody, messages.RandomGreeting(), callerID, "https://discord.gg/"+invite.Code)
```

- [ ] **Step 3: Build and run the full test suite**

Run: `go build ./... && go test ./...`
Expected: build succeeds, all tests pass (no existing test asserts on these template strings — confirmed via `grep -rln "messages\." --include=*_test.go .` returning nothing before this plan was written).

- [ ] **Step 4: Format and commit**

```bash
gofmt -w internal/messages/messages.go internal/commands/friend_add.go internal/party/invite.go
git add internal/messages/messages.go internal/commands/friend_add.go internal/party/invite.go
git commit -m "Randomize DM notice openers via RandomGreeting"
```

---

### Task 3: Fixed Subnautica opener for `PartyCreated`

**Files:**
- Modify: `internal/messages/messages.go:111` (`PartyCreated`)

**Interfaces:**
- None — this is a standalone literal string change, no new params, `internal/party/create.go:83`'s call (`fmt.Sprintf(messages.PartyCreated, ownerID)`) is unchanged since the arg count stays at one (`%d` for `ownerID`).

- [ ] **Step 1: Update the `PartyCreated` constant**

Change (line 111):
```go
const PartyCreated = "## Salutations <@%d>.\nThis channel is your designated party venue, currently operating in \"Friends of Friends\" mode.\n\nBe advised of the following:\n* Your friends are permitted to see and join this channel automatically.\n* Access rights may be adjusted using `/party_mode` (you can limit the scope to friends-only _or_ make it invite-only if you hate your friends).\n* To allow _other_ people in you can use `/party_allow`, or `/party_block` to prevent your evil enemies from joining.\n* For additional instruction, refer to `/help`."
```
to:
```go
const PartyCreated = "## Welcome aboard, Captain <@%d>. All systems online.\nThis channel is your designated party venue, currently operating in \"Friends of Friends\" mode.\n\nBe advised of the following:\n* Your friends are permitted to see and join this channel automatically.\n* Access rights may be adjusted using `/party_mode` (you can limit the scope to friends-only _or_ make it invite-only if you hate your friends).\n* To allow _other_ people in you can use `/party_allow`, or `/party_block` to prevent your evil enemies from joining.\n* For additional instruction, refer to `/help`."
```

- [ ] **Step 2: Build and run the full test suite**

Run: `go build ./... && go test ./...`
Expected: build succeeds, all tests pass.

- [ ] **Step 3: Format and commit**

```bash
gofmt -w internal/messages/messages.go
git add internal/messages/messages.go
git commit -m "Give party-creation notice a fixed Subnautica-themed opener"
```
