package naming

import (
	"strings"
	"testing"
)

func TestGenerateFormat(t *testing.T) {
	name := Generate()
	parts := strings.Split(name, "-")
	if len(parts) != 2 {
		t.Fatalf("Generate() = %q, want two hyphen-joined words", name)
	}
	if parts[0] == "" || parts[1] == "" {
		t.Fatalf("Generate() = %q, want non-empty words", name)
	}
}
