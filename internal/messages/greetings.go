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
