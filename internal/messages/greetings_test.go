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
