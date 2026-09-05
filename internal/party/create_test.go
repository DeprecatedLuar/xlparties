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

	mode, hasPreset, err := resolveAccessMode(s, owner)
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

	mode, hasPreset, err := resolveAccessMode(s, 8008)
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
