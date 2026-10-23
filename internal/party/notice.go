package party

import "github.com/bwmarrin/discordgo"

// postNotice posts content to a party channel without notifying anyone it
// mentions.
func (m *Manager) postNotice(channelID, content string) error {
	_, err := m.session.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
		Content:         content,
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	})
	return err
}
