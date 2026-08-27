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

	mode, err := resolveAccessMode(s, owner)
	if err != nil {
		t.Fatalf("resolveAccessMode: %v", err)
	}
	if mode != store.AccessModeFriendsOnly {
		t.Fatalf("resolveAccessMode = %q, want %q", mode, store.AccessModeFriendsOnly)
	}
}

func TestResolveAccessModeFallsBackToDefault(t *testing.T) {
	s := openTestStore(t)

	mode, err := resolveAccessMode(s, 8008)
	if err != nil {
		t.Fatalf("resolveAccessMode: %v", err)
	}
	if mode != store.DefaultAccessMode {
		t.Fatalf("resolveAccessMode = %q, want %q", mode, store.DefaultAccessMode)
	}
}
