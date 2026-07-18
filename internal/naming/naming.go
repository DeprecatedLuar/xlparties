// Package naming generates display names for party channels.
package naming

import (
	_ "embed"
	"math/rand"
	"strings"
	"unicode"
)

//go:embed wordbanks/adjectives.txt
var adjectivesRaw string

//go:embed wordbanks/locations.txt
var locationsRaw string

//go:embed wordbanks/nouns.txt
var nounsRaw string

const maxNameLength = 27

var templates = []string{
	"The <Adjective> <Location> of <Noun>",
	"<Adjective> <Location> of <Noun>",
	"The <Location> of <Noun>",
	"<Location> of <Noun>",
	"The <Adjective> <Location>",
	"<Adjective> <Location>",
	"The <Location>",
	"<Location>",
}

var (
	adjectives = cleanLines(adjectivesRaw)
	locations  = cleanLines(locationsRaw)
	nouns      = cleanLines(nounsRaw)
)

func cleanLines(raw string) []string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	var result []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

func capitalize(s string) string {
	if s == "" {
		return ""
	}
	parts := strings.Split(s, "-")
	for i, part := range parts {
		if len(part) > 0 {
			runes := []rune(part)
			runes[0] = unicode.ToUpper(runes[0])
			parts[i] = string(runes)
		}
	}
	return strings.Join(parts, "-")
}

// Generate returns a random name matching one of the Terraria templates,
// retrying until it fits maxNameLength (27 characters).
// The generated name is normalized to lowercase with spaces replaced by hyphens.
func Generate() string {
	for {
		template := templates[rand.Intn(len(templates))]
		name := template
		if strings.Contains(name, "<Adjective>") {
			adj := adjectives[rand.Intn(len(adjectives))]
			name = strings.ReplaceAll(name, "<Adjective>", capitalize(adj))
		}
		if strings.Contains(name, "<Location>") {
			loc := locations[rand.Intn(len(locations))]
			name = strings.ReplaceAll(name, "<Location>", capitalize(loc))
		}
		if strings.Contains(name, "<Noun>") {
			noun := nouns[rand.Intn(len(nouns))]
			name = strings.ReplaceAll(name, "<Noun>", capitalize(noun))
		}

		// Normalize: lowercase and replace spaces with hyphens
		name = strings.ToLower(name)
		name = strings.ReplaceAll(name, " ", "-")

		if len(name) <= maxNameLength {
			return name
		}
	}
}
