package main

import (
	"context"
	"log"
	"os"
	"strings"
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

	wikiPaths := requireEnv("WIKI_PATHS")
	geminiKey := requireEnv("GEMINI_API_KEY")
	slackBotToken := requireEnv("SLACK_BOT_TOKEN")
	slackAppToken := requireEnv("SLACK_APP_TOKEN")

	sources := parseSources(wikiPaths)
	if len(sources) == 0 {
		log.Fatal("WIKI_PATHS must contain at least one entry (format: name:/path,...)")
	}

	loader := wiki.NewLoader(sources)
	if err := loader.Load(); err != nil {
		log.Fatalf("failed to load wiki: %v", err)
	}
	loader.StartRefresh(24 * time.Hour)

	claudeClient, err := claude.NewClient(geminiKey)
	if err != nil {
		log.Fatalf("failed to create gemini client: %v", err)
	}

	sourceNames := loader.SourceNames()
	multiSource := len(sourceNames) > 1

	askFn := func(ctx context.Context, source, question string) (string, error) {
		return claudeClient.Ask(ctx, loader.Context(source), question, multiSource && source == "")
	}

	handler := slackbot.NewHandler(slackBotToken, slackAppToken, askFn, sourceNames)

	log.Printf("knowledge bot starting with sources: %s", strings.Join(sourceNames, ", "))
	handler.Run()
}

// parseSources parses "name1:/path1,name2:/path2" into Source entries.
func parseSources(raw string) []wiki.Source {
	var sources []wiki.Source
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		// split on first colon only, so paths with colons are safe
		idx := strings.Index(entry, ":")
		if idx <= 0 {
			log.Printf("skipping malformed WIKI_PATHS entry %q (expected name:/path)", entry)
			continue
		}
		sources = append(sources, wiki.Source{
			Name: strings.TrimSpace(entry[:idx]),
			Path: strings.TrimSpace(entry[idx+1:]),
		})
	}
	return sources
}

func requireEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("required env var %s is not set", key)
	}
	return val
}
