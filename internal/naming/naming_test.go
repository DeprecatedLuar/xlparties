package naming

import (
	"strings"
	"testing"
)

func TestGenerateFormat(t *testing.T) {
	// Create lookup maps for faster word bank membership checking in test.
	adjMap := make(map[string]bool)
	for _, a := range adjectives {
		adjMap[strings.ToLower(a)] = true
	}
	locMap := make(map[string]bool)
	for _, l := range locations {
		locMap[strings.ToLower(l)] = true
	}
	nounMap := make(map[string]bool)
	for _, n := range nouns {
		nounMap[strings.ToLower(n)] = true
	}

	for i := 0; i < 1000; i++ {
		name := Generate()
		if len(name) > 27 {
			t.Fatalf("Generate() = %q has length %d, want <= 27", name, len(name))
		}
		if name == "" {
			t.Fatalf("Generate() returned an empty string")
		}

		matched := false

		// Check each of the 9 normalized templates:

		// 1. the-<adjective>-<location>-of-<noun>
		if strings.HasPrefix(name, "the-") && strings.Contains(name, "-of-") {
			parts := strings.SplitN(name[4:], "-of-", 2)
			if len(parts) == 2 {
				mid, noun := parts[0], parts[1]
				if nounMap[noun] {
					subParts := strings.Split(mid, "-")
					for j := 1; j <= len(subParts); j++ {
						adj := strings.Join(subParts[:j], "-")
						loc := strings.Join(subParts[j:], "-")
						if adjMap[adj] && locMap[loc] {
							matched = true
							break
						}
					}
				}
			}
		}

		// 2. <adjective>-<location>-of-<noun>
		if !matched && strings.Contains(name, "-of-") {
			parts := strings.SplitN(name, "-of-", 2)
			if len(parts) == 2 {
				mid, noun := parts[0], parts[1]
				if nounMap[noun] {
					subParts := strings.Split(mid, "-")
					for j := 1; j <= len(subParts); j++ {
						adj := strings.Join(subParts[:j], "-")
						loc := strings.Join(subParts[j:], "-")
						if adjMap[adj] && locMap[loc] {
							matched = true
							break
						}
					}
				}
			}
		}

		// 3. the-<location>-of-<noun>
		if !matched && strings.HasPrefix(name, "the-") && strings.Contains(name, "-of-") {
			parts := strings.SplitN(name[4:], "-of-", 2)
			if len(parts) == 2 {
				loc, noun := parts[0], parts[1]
				if locMap[loc] && nounMap[noun] {
					matched = true
				}
			}
		}

		// 4. <location>-of-<noun>
		if !matched && strings.Contains(name, "-of-") {
			parts := strings.SplitN(name, "-of-", 2)
			if len(parts) == 2 {
				loc, noun := parts[0], parts[1]
				if locMap[loc] && nounMap[noun] {
					matched = true
				}
			}
		}

		// 5. the-<adjective>-<location>
		if !matched && strings.HasPrefix(name, "the-") {
			mid := name[4:]
			subParts := strings.Split(mid, "-")
			for j := 1; j <= len(subParts); j++ {
				adj := strings.Join(subParts[:j], "-")
				loc := strings.Join(subParts[j:], "-")
				if adjMap[adj] && locMap[loc] {
					matched = true
					break
				}
			}
		}

		// 6. <adjective>-<location>
		if !matched {
			subParts := strings.Split(name, "-")
			for j := 1; j <= len(subParts); j++ {
				adj := strings.Join(subParts[:j], "-")
				loc := strings.Join(subParts[j:], "-")
				if adjMap[adj] && locMap[loc] {
					matched = true
					break
				}
			}
		}

		// 7. the-<location>
		if !matched && strings.HasPrefix(name, "the-") {
			loc := name[4:]
			if locMap[loc] {
				matched = true
			}
		}

		// 8. <location>
		if !matched && locMap[name] {
			matched = true
		}

		// 9. <adjective>-<noun>
		if !matched {
			subParts := strings.Split(name, "-")
			for j := 1; j <= len(subParts); j++ {
				adj := strings.Join(subParts[:j], "-")
				noun := strings.Join(subParts[j:], "-")
				if adjMap[adj] && nounMap[noun] {
					matched = true
					break
				}
			}
		}

		if !matched {
			t.Fatalf("Generate() = %q did not match any expected template or wordbank values", name)
		}
	}
}
