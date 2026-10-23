package commands

import (
	"reflect"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func userOpt(name, id string) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionUser,
		Value: id,
	}
}

func interactionWith(opts ...*discordgo.ApplicationCommandInteractionDataOption) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		Type: discordgo.InteractionApplicationCommand,
		Data: discordgo.ApplicationCommandInteractionData{Options: opts},
	}}
}

func TestUserOptionIDsDedupesInOrder(t *testing.T) {
	i := interactionWith(userOpt("user", "3"), userOpt("user2", "1"), userOpt("user3", "3"))
	got, err := userOptionIDs(i)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int64{3, 1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestUserOptionIDsRequiresOne(t *testing.T) {
	if _, err := userOptionIDs(interactionWith()); err == nil {
		t.Fatal("expected error for missing user option")
	}
}

func TestUserOptionsSlots(t *testing.T) {
	opts := userOptions("d")
	if len(opts) != maxTargets {
		t.Fatalf("got %d slots, want %d", len(opts), maxTargets)
	}
	if opts[0].Name != "user" || !opts[0].Required {
		t.Fatalf("first slot must be required %q, got %q required=%v", "user", opts[0].Name, opts[0].Required)
	}
	if opts[1].Name != "user2" || opts[1].Required {
		t.Fatalf("second slot must be optional user2, got %q required=%v", opts[1].Name, opts[1].Required)
	}
}

func TestTargetedCallReplyJoinsSkippedAndResults(t *testing.T) {
	call := targetedCall{targets: []int64{1, 2}, skipped: []string{"skip"}}
	got := call.reply(func(target int64) string { return string(rune('a' + target)) })
	if want := "skip\nb\nc"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
