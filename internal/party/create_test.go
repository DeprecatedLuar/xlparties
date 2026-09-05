package party

import (
	"testing"

	"xlparties/internal/store"
)

func TestResolveAccessModeUsesPresetWhenSet(t *testing.T) {
	s := openTestStore(t)

	const owner = int64(7007)
	if err := s.UpsertPreset(owner, store.AccessModeFriendsOnly); err != nil {
		t.Fatalf("UpsertPreset: %v", err)
	}

	mode, hasPreset, err := resolveAccessMode(s, owner, "")
	if err != nil {
		t.Fatalf("resolveAccessMode: %v", err)
	}
	if mode != store.AccessModeFriendsOnly {
		t.Fatalf("resolveAccessMode = %q, want %q", mode, store.AccessModeFriendsOnly)
	}
	if !hasPreset {
		t.Fatalf("resolveAccessMode hasPreset = false, want true")
	}
}

func TestResolveAccessModeFallsBackToDefault(t *testing.T) {
	s := openTestStore(t)

	mode, hasPreset, err := resolveAccessMode(s, 8008, "")
	if err != nil {
		t.Fatalf("resolveAccessMode: %v", err)
	}
	if mode != store.DefaultAccessMode {
		t.Fatalf("resolveAccessMode = %q, want %q", mode, store.DefaultAccessMode)
	}
	if hasPreset {
		t.Fatalf("resolveAccessMode hasPreset = true, want false")
	}
}

func TestResolveAccessModeOverrideWinsOverPreset(t *testing.T) {
	s := openTestStore(t)

	const owner = int64(7009)
	if err := s.UpsertPreset(owner, store.AccessModeFriendsOnly); err != nil {
		t.Fatalf("UpsertPreset: %v", err)
	}

	mode, hasPreset, err := resolveAccessMode(s, owner, store.AccessModeInviteOnly)
	if err != nil {
		t.Fatalf("resolveAccessMode: %v", err)
	}
	if mode != store.AccessModeInviteOnly {
		t.Fatalf("resolveAccessMode = %q, want %q", mode, store.AccessModeInviteOnly)
	}
	if !hasPreset {
		t.Fatalf("resolveAccessMode hasPreset = false, want true")
	}
}

func TestResolveAccessModeRejectsUnknownOverride(t *testing.T) {
	s := openTestStore(t)

	if _, _, err := resolveAccessMode(s, 9009, "not_a_real_mode"); err == nil {
		t.Fatalf("resolveAccessMode: expected error for unknown override, got nil")
	}
}

func TestResolveUserLimitOverrideWinsOverPreset(t *testing.T) {
	s := openTestStore(t)

	const owner = int64(7010)
	if err := s.UpsertPresetLimit(owner, 4); err != nil {
		t.Fatalf("UpsertPresetLimit: %v", err)
	}

	override := 8
	limit, err := resolveUserLimit(s, owner, &override)
	if err != nil {
		t.Fatalf("resolveUserLimit: %v", err)
	}
	if limit != 8 {
		t.Fatalf("resolveUserLimit = %d, want 8", limit)
	}
}

func TestResolveUserLimitFallsBackToPresetThenZero(t *testing.T) {
	s := openTestStore(t)

	const owner = int64(7011)
	if err := s.UpsertPresetLimit(owner, 6); err != nil {
		t.Fatalf("UpsertPresetLimit: %v", err)
	}

	limit, err := resolveUserLimit(s, owner, nil)
	if err != nil {
		t.Fatalf("resolveUserLimit: %v", err)
	}
	if limit != 6 {
		t.Fatalf("resolveUserLimit = %d, want 6", limit)
	}

	limit, err = resolveUserLimit(s, 8011, nil)
	if err != nil {
		t.Fatalf("resolveUserLimit: %v", err)
	}
	if limit != 0 {
		t.Fatalf("resolveUserLimit = %d, want 0", limit)
	}
}

func TestResolveUserLimitRejectsOutOfRangeOverride(t *testing.T) {
	s := openTestStore(t)

	tooHigh := maxUserLimit + 1
	if _, err := resolveUserLimit(s, 9010, &tooHigh); err == nil {
		t.Fatalf("resolveUserLimit: expected error for out-of-range override, got nil")
	}
}
