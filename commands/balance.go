package commands

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

func HandleBalanceCommand(s *discordgo.Session, i *discordgo.InteractionCreate, client *APIClient) {
	customer, err := getCustomer(s, i.GuildID, client)
	if err != nil {
		respondToInteraction(s, i, "## ⛔ Error\n```diff\n- Unable to fetch customer data\n```", true)
		return
	}

	if customer == nil || customer.CustomerID == "" {
		respondToInteraction(s, i, "## ⛔ Error - No Customer Found", true)
		return
	}

	userData, err := getUser(s, i.User, customer.CustomerID, client)
	if err != nil || userData == nil {
		respondToInteraction(s, i, "## ⛔ Error\n```diff\n- Unable to fetch user data\n```", true)
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:       "💰 Balance",
		Description: fmt.Sprintf("**%s**, you have **🪙 %d** coins!", i.User.Username, userData.Points),
		Color:       0x00ff00, // Green color
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Use /draw to get more coins!",
		},
	}

	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	}

	response.Data.Flags = discordgo.MessageFlagsEphemeral

	s.InteractionRespond(i.Interaction, response)
}
