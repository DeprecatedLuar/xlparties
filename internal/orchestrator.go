// Package internal wires the bot's components together and runs it.
package internal

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"

	"xlparties/internal/commands"
	"xlparties/internal/config"
	"xlparties/internal/party"
	"xlparties/internal/store"
)

const gatewayReadyTimeout = 10 * time.Second

// Run loads config, opens the store, connects to Discord, registers
// commands, and blocks until an interrupt/terminate signal is received.
func Run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	session, guildID, err := connect(cfg.DiscordToken)
	if err != nil {
		return fmt.Errorf("connect to discord: %w", err)
	}
	defer session.Close()

	if err := checkRequiredPermissions(session, guildID); err != nil {
		log.Printf("permission check failed: %v", err)
	}

	if _, err := commands.Register(session, guildID, st); err != nil {
		return fmt.Errorf("register commands: %w", err)
	}

	partyManager := party.NewManager(session, st, guildID)
	partyManager.Register()
	partyManager.WarnIfWatchChannelUnset()

	log.Printf("xlparties is running (guild=%s)", guildID)
	waitForShutdownSignal()
	log.Println("shutting down")
	return nil
}

// connect opens the gateway session and derives the single guild id from the
// READY event.
func connect(token string) (*discordgo.Session, string, error) {
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, "", fmt.Errorf("create session: %w", err)
	}
	session.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildVoiceStates

	ready := make(chan *discordgo.Ready, 1)
	session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		ready <- r
	})

	if err := session.Open(); err != nil {
		return nil, "", fmt.Errorf("open gateway connection: %w", err)
	}

	var r *discordgo.Ready
	select {
	case r = <-ready:
	case <-time.After(gatewayReadyTimeout):
		session.Close()
		return nil, "", fmt.Errorf("timed out waiting for READY event from gateway")
	}

	if len(r.Guilds) != 1 {
		session.Close()
		return nil, "", fmt.Errorf("expected exactly 1 guild (spec: single-guild bot), found %d", len(r.Guilds))
	}

	return session, r.Guilds[0].ID, nil
}

func waitForShutdownSignal() {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
}
