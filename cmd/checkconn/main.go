// checkconn verifies the bot token is valid, confirms gateway connectivity,
// and checks that the bot has the permissions the spec requires
// (Manage Channels, Manage Roles, Move Members, View Channels, Connect).
// It also performs a live create/edit/delete cycle on a scratch voice channel
// to confirm the permissions actually work, not just that the bits are set.
// This is a standalone diagnostic, not part of the bot itself.
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

const (
	gatewayReadyTimeout   = 10 * time.Second
	guildPayloadGraceTime = 2 * time.Second
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

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, reading from process environment")
	}

	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		log.Fatal("DISCORD_TOKEN is not set")
	}

	session, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalf("failed to create session: %v", err)
	}

	session.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildVoiceStates

	ready := make(chan *discordgo.Ready, 1)
	session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		ready <- r
	})

	if err := session.Open(); err != nil {
		log.Fatalf("failed to open gateway connection: %v", err)
	}
	defer session.Close()

	var r *discordgo.Ready
	select {
	case r = <-ready:
	case <-time.After(gatewayReadyTimeout):
		log.Fatal("timed out waiting for READY event from gateway")
	}

	fmt.Printf("connected as %s#%s (id=%s)\n", r.User.Username, r.User.Discriminator, r.User.ID)

	if len(r.Guilds) != 1 {
		log.Fatalf("expected exactly 1 guild (spec: single-guild bot), found %d", len(r.Guilds))
	}
	guildID := r.Guilds[0].ID

	// give the gateway a moment to deliver the full GUILD_CREATE payload
	time.Sleep(guildPayloadGraceTime)

	checkPermissions(session, guildID)
	checkLiveChannelOps(session, guildID)
}

func checkPermissions(s *discordgo.Session, guildID string) {
	fmt.Println("\n--- permission check ---")

	guild, err := s.Guild(guildID)
	if err != nil {
		log.Fatalf("failed to fetch guild: %v", err)
	}

	member, err := s.GuildMember(guildID, s.State.User.ID)
	if err != nil {
		log.Fatalf("failed to fetch bot member: %v", err)
	}

	roleByID := make(map[string]*discordgo.Role, len(guild.Roles))
	for _, role := range guild.Roles {
		roleByID[role.ID] = role
	}

	var perms int64
	var highestPosition int
	var highestRoleName string
	for _, roleID := range member.Roles {
		role, ok := roleByID[roleID]
		if !ok {
			continue
		}
		perms |= role.Permissions
		if role.Position > highestPosition {
			highestPosition = role.Position
			highestRoleName = role.Name
		}
	}
	// @everyone permissions always apply
	if everyone, ok := roleByID[guildID]; ok {
		perms |= everyone.Permissions
	}

	if perms&discordgo.PermissionAdministrator != 0 {
		fmt.Println("bot has Administrator — all permissions implied")
	} else {
		missing := requiredPermissions &^ perms
		if missing == 0 {
			fmt.Println("all required permissions present:")
		} else {
			fmt.Println("MISSING required permissions:")
		}
		for bit, name := range permissionNames {
			status := "OK"
			if perms&bit == 0 {
				status = "MISSING"
			}
			fmt.Printf("  [%-7s] %s\n", status, name)
		}
	}

	fmt.Printf("bot's highest role: %q (position %d)\n", highestRoleName, highestPosition)
	fmt.Println("roles above the bot (moving/disconnecting members with these roles will fail):")
	found := false
	for _, role := range guild.Roles {
		if role.Position > highestPosition && role.Name != "@everyone" {
			fmt.Printf("  - %q (position %d)\n", role.Name, role.Position)
			found = true
		}
	}
	if !found {
		fmt.Println("  (none — bot's role is at or near the top)")
	}
}

func checkLiveChannelOps(s *discordgo.Session, guildID string) {
	fmt.Println("\n--- live channel operation check ---")

	name := fmt.Sprintf("xlparties-permcheck-%d", time.Now().Unix())
	channel, err := s.GuildChannelCreateComplex(guildID, discordgo.GuildChannelCreateData{
		Name: name,
		Type: discordgo.ChannelTypeGuildVoice,
		PermissionOverwrites: []*discordgo.PermissionOverwrite{
			{
				ID:   guildID, // @everyone
				Type: discordgo.PermissionOverwriteTypeRole,
				Deny: discordgo.PermissionViewChannel,
			},
		},
	})
	if err != nil {
		fmt.Printf("FAILED to create scratch voice channel (Manage Channels/Manage Roles not working): %v\n", err)
		return
	}
	fmt.Printf("created scratch channel %q (id=%s) with an overwrite — Manage Channels + Manage Roles confirmed\n", name, channel.ID)

	_, err = s.ChannelEditComplex(channel.ID, &discordgo.ChannelEdit{
		PermissionOverwrites: []*discordgo.PermissionOverwrite{
			{
				ID:   guildID,
				Type: discordgo.PermissionOverwriteTypeRole,
				Deny: discordgo.PermissionViewChannel | discordgo.PermissionVoiceConnect,
			},
		},
	})
	if err != nil {
		fmt.Printf("FAILED to edit scratch channel overwrites: %v\n", err)
	} else {
		fmt.Println("edited scratch channel overwrites successfully")
	}

	if _, err := s.ChannelDelete(channel.ID); err != nil {
		fmt.Printf("FAILED to delete scratch channel — MANUAL CLEANUP NEEDED for channel id=%s: %v\n", channel.ID, err)
	} else {
		fmt.Println("deleted scratch channel — cleanup confirmed")
	}

	fmt.Println("\nnote: Move Members and Connect are checked by permission bit only above.")
	fmt.Println("Testing an actual member move requires a real user connected to a voice channel.")
}
