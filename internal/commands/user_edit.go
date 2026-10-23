package commands

import (
	"github.com/bwmarrin/discordgo"

	"xlparties/internal/logger"
	"xlparties/internal/party"
)

// runUserEdit is the shared body of the relationship commands: it resolves
// the targets, applies action to each, rebuilds the caller's affected
// channels once, and sends one ephemeral reply. action returns the reply
// line for its target.
func runUserEdit(s *discordgo.Session, i *discordgo.InteractionCreate, pm *party.Manager, command string, action func(caller, target int64) string) {
	call, ok := callerAndTargets(s, i)
	if !ok {
		return
	}
	reply := call.reply(func(target int64) string { return action(call.caller, target) })
	if err := pm.RewriteAffectedChannels(call.caller); err != nil {
		logger.Error(command+": rewrite affected channels", "caller", call.caller, "error", err)
	}
	respondEphemeral(s, i, reply)
}
