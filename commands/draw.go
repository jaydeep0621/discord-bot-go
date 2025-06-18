package commands

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"

	"github.com/bwmarrin/discordgo"
)

type Quest struct {
	QuestID string `json:"questId"`
	Status  string `json:"status"`
	Steps   struct {
		Rewards []Reward `json:"rewards"`
		Final   struct {
			AdvanceOptions struct {
				RedirectChannelID string `json:"redirect_channel_id"`
			} `json:"advance_options"`
		} `json:"final"`
	} `json:"steps"`
}

type Reward struct {
	ItemIds     []string `json:"itemIds"`
	Probability float64  `json:"probability"`
	Points      int      `json:"points"`
}

type Item struct {
	ID             string `json:"_id"`
	NameEng        string `json:"name_eng"`
	NameJp         string `json:"name_jp"`
	DescriptionEng string `json:"description_eng"`
	DescriptionJp  string `json:"description_jp"`
	ImageURL       string `json:"image_url"`
}

type UserData struct {
	UserID string `json:"userId"`
	Points int    `json:"points"`
}

type Customer struct {
	CustomerID string `json:"customerId"`
}

type Event struct {
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

type CardsResponse struct {
	QuestID string   `json:"questId"`
	Cards   []Reward `json:"cards"`
	Items   []Item   `json:"items"`
	Error   string   `json:"error,omitempty"`
}

var DrawButtonMessages = map[string]map[string]string{
	"0": { 
		"drawing": "🎲 Drawing your card...",
	},
	"1": { 
		"drawing": "🎲 カードを引いています...",
	},
}

var ClaimType = map[string]string{
	"0": "lastClaimAtUs",
	"1": "lastClaimAtJapanese",
}

func HandleDrawCommand(s *discordgo.Session, i *discordgo.InteractionCreate, client *APIClient) {
	language := "0"
	logChannelID := ""
	suffix := "eng"

	if strings.Contains(i.GuildID, "jp") {
		language = "1"
		suffix = "jp"
	}

	messages := DrawButtonMessages[language]

	respondToInteraction(s, i, messages["drawing"], true)

	channelID := i.ChannelID

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

	cards, err := getCards(userData.UserID, customer.CustomerID, channelID, client)
	if err != nil {
		respondToInteraction(s, i, "## ⛔ Error\n```diff\n- Unable to fetch cards\n```", true)
		return
	}

	if cards.Error != "" {
		respondToInteraction(s, i, fmt.Sprintf("## ⛔ Error\n```diff\n- %s\n```", cards.Error), true)
		return
	}

	chosenIndex := chooseCumulativeRarity(toCumulativeProbabilities(cards.Cards))
	if chosenIndex < 0 || chosenIndex >= len(cards.Cards) {
		respondToInteraction(s, i, "## ⛔ Cards Not Available", true)
		return
	}

	card := cards.Cards[chosenIndex]
	if len(card.ItemIds) == 0 {
		respondToInteraction(s, i, "## ⛔ No Item Available in Card", true)
		return
	}

	randomItemID := card.ItemIds[rand.Intn(len(card.ItemIds))]
	var chosenItem *Item
	for _, item := range cards.Items {
		if item.ID == randomItemID {
			chosenItem = &item
			break
		}
	}

	if chosenItem == nil {
		respondToInteraction(s, i, "## ⛔ Items Not Available", true)
		return
	}

	events, err := questClaimed(customer.CustomerID, userData.UserID, cards.QuestID, client)
	if err == nil && events != nil {
		respondToInteraction(s, i, "## ⛔ Already Claimed\n```diff\n-Card already drawn for today\n```", true)
		return
	}

	questData, err := getQuestData(customer.CustomerID, channelID, cards.QuestID, client)
	if err != nil || questData == nil {
		respondToInteraction(s, i, "## ❌ Quests Not Available", true)
		return
	}

	if questData.Steps.Final.AdvanceOptions.RedirectChannelID != "" {
		logChannelID = questData.Steps.Final.AdvanceOptions.RedirectChannelID
	}

	points := userData.Points + card.Points

	var itemName, itemDescription string
	if suffix == "jp" {
		itemName = chosenItem.NameJp
		itemDescription = chosenItem.DescriptionJp
	} else {
		itemName = chosenItem.NameEng
		itemDescription = chosenItem.DescriptionEng
	}

	content := fmt.Sprintf("# %s, You earned 🪙 %d!\nCurrent 🪙 coins owned: %d\n## %s",
		i.User.Username, card.Points, points, itemName)

	if itemDescription != "" {
		content += "\n" + itemDescription
	}

	var embeds []*discordgo.MessageEmbed
	if chosenItem.ImageURL != "" {
		embeds = append(embeds, &discordgo.MessageEmbed{
			Color: rand.Intn(0xFFFFFF),  
			Image: &discordgo.MessageEmbedImage{
				URL: chosenItem.ImageURL,
			},
		})
	}

	if logChannelID != "" {
		_, err := s.Channel(logChannelID)
		if err == nil {
			s.ChannelMessageSendComplex(logChannelID, &discordgo.MessageSend{
				Content: content,
				Embeds:  embeds,
			})

			respondToInteraction(s, i, fmt.Sprintf("Check out the result in <#%s>", logChannelID), false)
		} else {
			respondToInteraction(s, i, content, false)
		}
	} else {
		respondToInteraction(s, i, content, false)
	}

	go createEvent(map[string]interface{}{
		"questId":    cards.QuestID,
		"userId":     userData.UserID,
		"itemId":     chosenItem.ID,
		"customerId": customer.CustomerID,
	}, client)

	go createEvent(map[string]interface{}{
		"questId":    cards.QuestID,
		"userId":     userData.UserID,
		"points":     card.Points,
		"customerId": customer.CustomerID,
	}, client)

	go updateUser(i.User.ID, userData.UserID, points, chosenItem.ID, client)
}

type APIClient struct {
	BaseURL    string
	AuthToken  string
	HTTPClient *http.Client
}

func getCustomer(s *discordgo.Session, guildID string, client *APIClient) (*Customer, error) {
	url := fmt.Sprintf("%s/api/v1/customer?discordAccountId=%s", client.BaseURL, guildID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if client.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.AuthToken)
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var customer Customer
	if err := json.NewDecoder(resp.Body).Decode(&customer); err != nil {
		return nil, err
	}

	return &customer, nil
}

func getUser(s *discordgo.Session, user *discordgo.User, customerID string, client *APIClient) (*UserData, error) {
	url := fmt.Sprintf("%s/api/v1/user?discordId=%s&customerId=%s", client.BaseURL, user.ID, customerID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if client.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.AuthToken)
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var userData UserData
	if err := json.NewDecoder(resp.Body).Decode(&userData); err != nil {
		return nil, err
	}

	if userData.UserID == "" {
		return createUser(s, user, customerID, client)
	}

	return &userData, nil
}

func createUser(s *discordgo.Session, user *discordgo.User, customerID string, client *APIClient) (*UserData, error) {
	url := fmt.Sprintf("%s/api/v1/user", client.BaseURL)

	userData := map[string]interface{}{
		"discordId":  user.ID,
		"name":       user.Username,
		"customerId": customerID,
	}

	jsonData, err := json.Marshal(userData)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if client.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.AuthToken)
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var newUser UserData
	if err := json.NewDecoder(resp.Body).Decode(&newUser); err != nil {
		return nil, err
	}

	return &newUser, nil
}

func getCards(userID, customerID, channelID string, client *APIClient) (*CardsResponse, error) {
	url := fmt.Sprintf("%s/api/v1/quests?customerId=%s&socialNetworkId=%s", client.BaseURL, customerID, channelID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if client.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.AuthToken)
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var questsResponse struct {
		Quests []Quest `json:"quests"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&questsResponse); err != nil {
		return nil, err
	}

	var selectedQuest *Quest
	for _, quest := range questsResponse.Quests {
		if quest.Status == "active" {
			selectedQuest = &quest
			break
		}
	}

	if selectedQuest == nil {
		return &CardsResponse{Error: "No Active Quests"}, nil
	}

	if len(selectedQuest.Steps.Rewards) == 0 {
		return &CardsResponse{Error: "No Rewards"}, nil
	}

	var itemIDs []string
	for _, reward := range selectedQuest.Steps.Rewards {
		itemIDs = append(itemIDs, reward.ItemIds...)
	}

	if len(itemIDs) == 0 {
		return &CardsResponse{Error: "Quest doesn't contain only items to win :("}, nil
	}

	itemIDsQuery := strings.Join(itemIDs, ",")
	itemsURL := fmt.Sprintf("%s/api/v1/items?itemIds=%s", client.BaseURL, itemIDsQuery)

	itemsReq, err := http.NewRequest("GET", itemsURL, nil)
	if err != nil {
		return nil, err
	}

	if client.AuthToken != "" {
		itemsReq.Header.Set("Authorization", "Bearer "+client.AuthToken)
	}

	itemsResp, err := client.HTTPClient.Do(itemsReq)
	if err != nil {
		return nil, err
	}
	defer itemsResp.Body.Close()

	var items []Item
	if err := json.NewDecoder(itemsResp.Body).Decode(&items); err != nil {
		return nil, err
	}

	return &CardsResponse{
		QuestID: selectedQuest.QuestID,
		Cards:   selectedQuest.Steps.Rewards,
		Items:   items,
	}, nil
}

func getQuestData(customerID, channelID, questID string, client *APIClient) (*Quest, error) {
	var url string
	if questID != "" {
		url = fmt.Sprintf("%s/api/v1/quests?customerId=%s&socialNetworkId=%s&questId=%s", client.BaseURL, customerID, channelID, questID)
	} else {
		url = fmt.Sprintf("%s/api/v1/quests?customerId=%s&socialNetworkId=%s", client.BaseURL, customerID, channelID)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if client.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.AuthToken)
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var questsResponse struct {
		Quests []Quest `json:"quests"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&questsResponse); err != nil {
		return nil, err
	}

	for _, quest := range questsResponse.Quests {
		if quest.Status == "active" {
			return &quest, nil
		}
	}

	return nil, nil
}

func questClaimed(customerID, userID, questID string, client *APIClient) (*Event, error) {
	url := fmt.Sprintf("%s/api/v1/event?customerId=%s&userId=%s&questId=%s", client.BaseURL, customerID, userID, questID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if client.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.AuthToken)
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var eventsResponse struct {
		Events []Event `json:"events"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&eventsResponse); err != nil {
		return nil, err
	}

	for _, event := range eventsResponse.Events {
		if event.Status == "processed" {
			return &event, nil
		}
	}

	return nil, nil
}

func createEvent(eventData map[string]interface{}, client *APIClient) error {
	url := fmt.Sprintf("%s/api/v1/event", client.BaseURL)

	jsonData, err := json.Marshal(eventData)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if client.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.AuthToken)
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func updateUser(discordID, userID string, points int, itemID string, client *APIClient) error {
	url := fmt.Sprintf("%s/api/v1/user/%s", client.BaseURL, userID)

	userData := map[string]interface{}{
		"discordId": discordID,
		"points":    points,
		"items": []map[string]interface{}{
			{
				"itemId": itemID,
			},
		},
	}

	jsonData, err := json.Marshal(userData)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	if client.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.AuthToken)
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func toCumulativeProbabilities(rewards []Reward) []float64 {
	cumulative := make([]float64, 0, len(rewards))
	var last float64

	for _, reward := range rewards {
		cumulative = append(cumulative, last+reward.Probability)
		last = cumulative[len(cumulative)-1]
	}

	return cumulative
}

func chooseCumulativeRarity(cumulativeProbabilities []float64) int {
	if len(cumulativeProbabilities) == 0 {
		return -1
	}

	option := rand.Float64() * cumulativeProbabilities[len(cumulativeProbabilities)-1]

	for i, prob := range cumulativeProbabilities {
		if option < prob {
			return i
		}
	}

	return -1
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
