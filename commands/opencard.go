package commands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func HandleOpenCardCommand(s *discordgo.Session, i *discordgo.InteractionCreate, client *APIClient) {
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
		Title:       "🎴 Open Card",
		Description: fmt.Sprintf("**%s**, you can open your cards here!", i.User.Username),
		Color:       0x0099ff,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Card opening feature coming soon!",
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

func HandleOpenCardButton(s *discordgo.Session, i *discordgo.InteractionCreate, customID string) {
	parts := strings.Split(customID, "-")
	if len(parts) != 3 {
		respondToInteraction(s, i, "Invalid button interaction", true)
		return
	}

	userID := parts[1]
	pageStr := parts[2]

	if userID != i.User.ID {
		respondToInteraction(s, i, "This is not your card collection!", true)
		return
	}

	page, err := strconv.Atoi(pageStr)
	if err != nil {
		respondToInteraction(s, i, "Invalid page number", true)
		return
	}

	embed := &discordgo.MessageEmbed{
		Title:       "🎴 Your Cards",
		Description: fmt.Sprintf("**%s**, viewing page %d of your cards", i.User.Username, page+1),
		Color:       0x0099ff,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Card browsing feature coming soon!",
		},
	}

	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	}

	s.InteractionRespond(i.Interaction, response)
}
