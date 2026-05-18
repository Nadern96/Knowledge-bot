package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"

	"knowledge-bot/internal/claude"
	slackbot "knowledge-bot/internal/slack"
	"knowledge-bot/internal/wiki"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file, reading from environment")
	}

	wikiPath := requireEnv("WIKI_PATH")
	geminiKey := requireEnv("GEMINI_API_KEY")
	slackBotToken := requireEnv("SLACK_BOT_TOKEN")
	slackAppToken := requireEnv("SLACK_APP_TOKEN")

	loader := wiki.NewLoader(wikiPath)
	if err := loader.Load(); err != nil {
		log.Fatalf("failed to load wiki: %v", err)
	}
	loader.StartRefresh(24 * time.Hour)

	claudeClient, err := claude.NewClient(geminiKey)
	if err != nil {
		log.Fatalf("failed to create gemini client: %v", err)
	}

	askFn := func(ctx context.Context, question string) (string, error) {
		return claudeClient.Ask(ctx, loader.Context(), question)
	}

	handler := slackbot.NewHandler(slackBotToken, slackAppToken, askFn)

	log.Println("knowledge bot starting...")
	handler.Run()
}

func requireEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("required env var %s is not set", key)
	}
	return val
}
