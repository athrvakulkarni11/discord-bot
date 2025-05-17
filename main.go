package main

import (
	"bytes"
	"context"
	"encoding/json"
	// "fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Fatal("❌ Error loading .env file")
	}

	token := os.Getenv("DISCORD_BOT_TOKEN")
	if token == "" {
		log.Fatal("❌ DISCORD_BOT_TOKEN is not set")
	}

	// Create Discord session
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalf("❌ Error creating Discord session: %v", err)
	}

	dg.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsMessageContent
	dg.AddHandler(messageHandler)

	// Open connection
	if err := dg.Open(); err != nil {
		log.Fatalf("❌ Error opening Discord connection: %v", err)
	}
	defer dg.Close()

	log.Println("✅ Bot is now running. Press CTRL+C to exit.")

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("👋 Shutting down...")
}

func messageHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot {
		return
	}

	content := m.Content
	var input, prompt, response string

	switch {
	case strings.HasPrefix(content, "/summarize"):
		input = strings.TrimPrefix(content, "/summarize ")
		prompt = "Summarize this:\n" + input

	case strings.HasPrefix(content, "/explain"):
		input = strings.TrimPrefix(content, "/explain ")
		prompt = "Explain this:\n" + input

	case strings.HasPrefix(content, "/translate"):
		input = strings.TrimPrefix(content, "/translate ")
		prompt = "Translate this to English:\n" + input

	default:
		return
	}

	response = callGroq(prompt)
	s.ChannelMessageSend(m.ChannelID, response)
}

func callGroq(prompt string) string {
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return "❌ GROQ_API_KEY is not set in .env"
	}

	client := &http.Client{Timeout: 20 * time.Second}
	ctx := context.Background()

	body := map[string]interface{}{
		"model": "llama3-70b-8192",
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
	}

	jsonData, err := json.Marshal(body)
	if err != nil {
		return "❌ Failed to encode request"
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "❌ Failed to create Groq API request"
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "❌ Error contacting Groq API"
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "❌ Failed to read Groq response"
	}

	var result map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "❌ Failed to parse Groq response"
	}

	choices, ok := result["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return "❌ No response from Groq model"
	}

	message, ok := choices[0].(map[string]interface{})["message"].(map[string]interface{})["content"].(string)
	if !ok {
		return "❌ Groq response missing content"
	}

	return message
}
