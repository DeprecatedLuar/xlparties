// Package naming generates display names for party channels.
package naming

import "math/rand"

var adjectives = []string{
	"amber", "brave", "calm", "cosmic", "crimson", "dizzy", "eager", "electric",
	"fuzzy", "gentle", "golden", "happy", "hidden", "icy", "jolly", "keen",
	"lively", "lucky", "misty", "mellow", "neon", "nimble", "orbiting", "polished",
	"quiet", "quick", "rapid", "rusty", "sandy", "shiny", "silent", "silver",
	"sleepy", "sneaky", "solar", "sunny", "swift", "tidy", "velvet", "witty",
}

var nouns = []string{
	"aurora", "badger", "beacon", "canyon", "comet", "coral", "coyote", "delta",
	"dune", "eagle", "ember", "falcon", "fjord", "forest", "galaxy", "glacier",
	"harbor", "island", "jaguar", "lagoon", "lantern", "lynx", "meadow", "meteor",
	"nebula", "oasis", "otter", "panther", "phoenix", "prairie", "raven", "reef",
	"ridge", "river", "summit", "tundra", "valley", "willow", "wolf", "zephyr",
}

// Generate returns a random two-word "adjective-noun" name, e.g. "amber-falcon".
func Generate() string {
	return adjectives[rand.Intn(len(adjectives))] + "-" + nouns[rand.Intn(len(nouns))]
}
