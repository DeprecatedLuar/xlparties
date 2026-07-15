package internal

import (
	"log"

	"github.com/bwmarrin/discordgo"
)

const requiredPermissions = discordgo.PermissionManageChannels |
	discordgo.PermissionManageRoles |
	discordgo.PermissionVoiceMoveMembers |
	discordgo.PermissionViewChannel |
	discordgo.PermissionVoiceConnect

var permissionNames = map[int64]string{
	discordgo.PermissionManageChannels:   "Manage Channels",
	discordgo.PermissionManageRoles:      "Manage Roles",
	discordgo.PermissionVoiceMoveMembers: "Move Members",
	discordgo.PermissionViewChannel:      "View Channels",
	discordgo.PermissionVoiceConnect:     "Connect",
}

// checkRequiredPermissions logs loudly if the bot lacks any permission the
// spec requires, without failing startup (Administrator satisfies all of
// them; if admin is ever dropped this is what catches a lockout early).
func checkRequiredPermissions(s *discordgo.Session, guildID string) error {
	guild, err := s.Guild(guildID)
	if err != nil {
		return err
	}
	member, err := s.GuildMember(guildID, s.State.User.ID)
	if err != nil {
		return err
	}

	roleByID := make(map[string]*discordgo.Role, len(guild.Roles))
	for _, role := range guild.Roles {
		roleByID[role.ID] = role
	}

	var perms int64
	for _, roleID := range member.Roles {
		if role, ok := roleByID[roleID]; ok {
			perms |= role.Permissions
		}
	}
	if everyone, ok := roleByID[guildID]; ok {
		perms |= everyone.Permissions
	}

	if perms&discordgo.PermissionAdministrator != 0 {
		log.Println("bot has Administrator — all required permissions present")
		return nil
	}

	missing := requiredPermissions &^ perms
	if missing == 0 {
		log.Println("all required permissions present")
		return nil
	}

	log.Println("WARNING: bot is missing required permissions:")
	for bit, name := range permissionNames {
		if missing&bit != 0 {
			log.Printf("  MISSING: %s", name)
		}
	}
	return nil
}
