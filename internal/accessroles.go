package internal

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/config"
	"xlparties/internal/logger"
)

// resolveAlwaysAllowedRoles turns the role names configured in
// ALWAYS_ALLOWED_ROLES into role ids by looking them up in the live guild.
// Names are matched case-insensitively.
//
// It hard-fails on a name that matches no role, or more than one: a role that
// was renamed or deleted would otherwise silently stop being admitted to new
// parties, which is exactly the lockout this setting exists to prevent.
//
// @everyone is rejected outright - the overwrite builder owns that entry, and
// allowing it here would flip every party channel public.
func resolveAlwaysAllowedRoles(s *discordgo.Session, guildID string, names []string) ([]string, error) {
	if len(names) == 0 {
		return nil, nil
	}

	roles, err := s.GuildRoles(guildID)
	if err != nil {
		return nil, fmt.Errorf("list guild roles: %w", err)
	}

	ids := make([]string, 0, len(names))
	for _, name := range names {
		id, err := roleIDByName(roles, guildID, name)
		if err != nil {
			return nil, err
		}
		logger.Info("role always allowed in party channels", "role", name, "id", id)
		ids = append(ids, id)
	}
	return ids, nil
}

func roleIDByName(roles []*discordgo.Role, guildID, name string) (string, error) {
	var matches []*discordgo.Role
	for _, role := range roles {
		if strings.EqualFold(role.Name, name) {
			matches = append(matches, role)
		}
	}

	switch {
	case len(matches) == 0:
		return "", fmt.Errorf("%s names role %q, which does not exist in the guild", config.AlwaysAllowedRolesEnv, name)
	case len(matches) > 1:
		return "", fmt.Errorf("%s names role %q, which matches %d roles - rename one so it is unambiguous", config.AlwaysAllowedRolesEnv, name, len(matches))
	case matches[0].ID == guildID:
		return "", fmt.Errorf("%s cannot name @everyone - that would make every party public", config.AlwaysAllowedRolesEnv)
	}
	return matches[0].ID, nil
}
