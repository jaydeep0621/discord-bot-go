package commands

import (
	"log"

	"github.com/bwmarrin/discordgo"
)

func RegisterCommands(s *discordgo.Session, guildID string) error {
	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "draw",
			Description: "Draw a card and earn coins!",
		},
		{
			Name:        "balance",
			Description: "Check your coin balance",
		},
		{
			Name:        "opencard",
			Description: "Open your card collection",
		},
	}

	log.Println("Registering commands...")

	for _, cmd := range commands {
		_, err := s.ApplicationCommandCreate(s.State.User.ID, guildID, cmd)
		if err != nil {
			log.Printf("Error creating command %s: %v", cmd.Name, err)
			return err
		}
		log.Printf("Registered command: %s", cmd.Name)
	}

	return nil
}

func UnregisterCommands(s *discordgo.Session, guildID string) error {
	commands, err := s.ApplicationCommands(s.State.User.ID, guildID)
	if err != nil {
		return err
	}

	for _, cmd := range commands {
		err := s.ApplicationCommandDelete(s.State.User.ID, guildID, cmd.ID)
		if err != nil {
			log.Printf("Error deleting command %s: %v", cmd.Name, err)
		} else {
			log.Printf("Unregistered command: %s", cmd.Name)
		}
	}

	return nil
}
