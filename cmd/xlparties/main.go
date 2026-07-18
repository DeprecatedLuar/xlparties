// Command xlparties runs the party VC bot. This entry point handles only
// process concerns (exit code); all wiring lives in the internal package.
package main

import (
	"xlparties/internal"
	"xlparties/internal/logger"
)

func main() {
	if err := internal.Run(); err != nil {
		logger.Fatal("xlparties exited", "error", err)
	}
}
