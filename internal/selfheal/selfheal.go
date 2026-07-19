// Package selfheal sequences startup reconciliation checks that repair state
// drift no single package can detect from its own data alone (e.g. Discord
// objects created but never persisted because the process died mid-write).
// It holds no Discord or DB logic itself; it only calls exported functions on
// other packages, the same way orchestrator sequences packages.
package selfheal

import "xlparties/internal/party"

// Run performs all startup self-heal checks. Called once, after the owning
// package's own reconciliation (e.g. party.StartupSweep) has run.
func Run(partyManager *party.Manager) {
	partyManager.ReconcileStaleCreations()
	partyManager.SweepOrphanChannels()
}
