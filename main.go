package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"

	"discord-bot/commands"
)

type Card struct {
	Name   string `json:"name"`
	Rarity int    `json:"rarity"`
	Coins  int    `json:"coins"`
	Image  string `json:"image"`
	Artist Artist `json:"artist"`
}

type Artist struct {
	Name    string            `json:"name"`
	Avatar  string            `json:"avatar"`
	Socials map[string]string `json:"socials"`
	Comment string            `json:"comment"`
	Channel string            `json:"channel"`
}

type User struct {
	Coins int    `json:"coins"`
	Cards []Card `json:"cards"`
}

type Bot struct {
	Session *discordgo.Session
	Client  *Client
}

type Client struct {
	BaseURL    string
	AuthToken  string
	HTTPClient *http.Client
}

var (
	bot *Bot
	CARDS = []Card{
		{
			Name:   "Kard Ichi",
			Rarity: 2,
			Coins:  2,
			Image:  "https://cdn.discordapp.com/attachments/1177859810889310218/1205794963934289990/Ice_near.jpg?ex=65e2e591&is=65d07091&hm=19b779891601aa1960d14cec0e924f9878bbf0589b954e97bffbfddccac9e80c",
			Artist: Artist{
				Name:   "Shi Sama",
				Avatar: "https://cdn.discordapp.com/attachments/723104565708324915/1207006657335533638/image.png?ex=65e74e0c&is=65d4d90c&hm=9084deeef3a72ab1e6fd37bab7db3db4f45bc9dbf28a760a44a371ec361e4cfd&",
				Socials: map[string]string{
					"X":        "https://twitter.com/krazy_shisui",
					"You Tube": "https://www.youtube.com/krazydeveloper",
				},
				Comment: "Man this is fine isn;t it?",
				Channel: "957647248144105522",
			},
		},
	}
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	client := &Client{
		BaseURL:    getEnv("BASE_URI", "http://localhost:3002"),
		AuthToken:  getEnv("API_AUTH", ""),
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}

	session, err := discordgo.New("Bot " + getEnv("TOKEN", ""))
	if err != nil {
		log.Fatal("Error creating Discord session: ", err)
	}

	bot = &Bot{
		Session: session,
		Client:  client,
	}

	setupDiscordHandlers(session)
}

func main() {
	go startHTTPServer()

	if err := bot.Session.Open(); err != nil {
		log.Fatal("Error opening Discord connection: ", err)
	}
	defer bot.Session.Close()

	for _, guild := range bot.Session.State.Guilds {
		if err := commands.RegisterCommands(bot.Session, guild.ID); err != nil {
			log.Printf("Error registering commands for guild %s: %v", guild.Name, err)
		}
	}

	log.Println("Bot is now running. Press CTRL-C to exit.")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	log.Println("Shutting down...")

	for _, guild := range bot.Session.State.Guilds {
		if err := commands.UnregisterCommands(bot.Session, guild.ID); err != nil {
			log.Printf("Error unregistering commands for guild %s: %v", guild.Name, err)
		}
	}
}

func startHTTPServer() {
	r := mux.NewRouter()

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	r.HandleFunc("/api/v1/cards", getCardsHandler).Methods("GET")
	r.HandleFunc("/api/v1/users/{id}", getUserHandler).Methods("GET")

	port := getEnv("PORT", "3000")
	log.Printf("HTTP server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func getCardsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(CARDS)
}

func getUserHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	_ = vars["id"] 

	user := User{
		Coins: 10,
		Cards: CARDS[:2], 
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func setupDiscordHandlers(s *discordgo.Session) {
	s.AddHandler(onReady)
	s.AddHandler(onGuildCreate)
	s.AddHandler(onInteractionCreate)
}

func onReady(s *discordgo.Session, event *discordgo.Ready) {
	log.Printf("Bot is ready! Logged in as: %s#%s", s.State.User.Username, s.State.User.Discriminator)
}

func onGuildCreate(s *discordgo.Session, event *discordgo.GuildCreate) {
	log.Printf("Joined a new server: %s (id: %s)", event.Guild.Name, event.Guild.ID)
}

func onInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type == discordgo.InteractionApplicationCommand {
		handleSlashCommand(s, i)
	}

	if i.Type == discordgo.InteractionMessageComponent {
		handleButtonInteraction(s, i)
	}
}

func handleSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	commandName := i.ApplicationCommandData().Name

	apiClient := &commands.APIClient{
		BaseURL:    bot.Client.BaseURL,
		AuthToken:  bot.Client.AuthToken,
		HTTPClient: bot.Client.HTTPClient,
	}

	switch commandName {
	case "draw":
		commands.HandleDrawCommand(s, i, apiClient)
	case "balance":
		commands.HandleBalanceCommand(s, i, apiClient)
	case "opencard":
		commands.HandleOpenCardCommand(s, i, apiClient)
	default:
		respondToInteraction(s, i, "Unknown command", true)
	}
}

func handleButtonInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	customID := i.MessageComponentData().CustomID

	if len(customID) >= 9 && customID[:9] == "openCard-" {
		commands.HandleOpenCardButton(s, i, customID)
	}
}

func respondToInteraction(s *discordgo.Session, i *discordgo.InteractionCreate, content string, ephemeral bool) {
	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
		},
	}

	if ephemeral {
		response.Data.Flags = discordgo.MessageFlagsEphemeral
	}

	s.InteractionRespond(i.Interaction, response)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
