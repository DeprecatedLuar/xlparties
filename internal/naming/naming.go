// Package naming generates display names for party channels.
package naming

import (
	_ "embed"
	"math/rand"
	"strings"
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
	"<Adjective> <Noun>",
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

// Generate returns a random name matching one of the Terraria templates,
// retrying until it fits maxNameLength (27 characters). The result is
// normalized to lowercase with spaces replaced by hyphens.
func Generate() string {
	for {
		name := templates[rand.Intn(len(templates))]
		if strings.Contains(name, "<Adjective>") {
			name = strings.ReplaceAll(name, "<Adjective>", adjectives[rand.Intn(len(adjectives))])
		}
		if strings.Contains(name, "<Location>") {
			name = strings.ReplaceAll(name, "<Location>", locations[rand.Intn(len(locations))])
		}
		if strings.Contains(name, "<Noun>") {
			name = strings.ReplaceAll(name, "<Noun>", nouns[rand.Intn(len(nouns))])
		}

		name = strings.ToLower(name)
		name = strings.ReplaceAll(name, " ", "-")

		if len(name) <= maxNameLength {
			return name
		}
	}
}
