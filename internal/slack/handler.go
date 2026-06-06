package slack

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// AskFunc is the function the handler calls to get an answer.
// source is the name of the wiki source to query; empty means all sources.
type AskFunc func(ctx context.Context, source, question string) (string, error)

type Handler struct {
	client       *slack.Client
	socket       *socketmode.Client
	askFn        AskFunc
	sourceNames  []string
}

func NewHandler(botToken, appToken string, askFn AskFunc, sourceNames []string) *Handler {
	client := slack.New(
		botToken,
		slack.OptionAppLevelToken(appToken),
	)
	socket := socketmode.New(client)
	return &Handler{
		client:      client,
		socket:      socket,
		askFn:       askFn,
		sourceNames: sourceNames,
	}
}

func (h *Handler) Run() {
	go h.processEvents()
	if err := h.socket.Run(); err != nil {
		log.Fatalf("socket mode error: %v", err)
	}
}

func (h *Handler) processEvents() {
	for evt := range h.socket.Events {
		switch evt.Type {
		case socketmode.EventTypeEventsAPI:
			h.socket.Ack(*evt.Request)
			apiEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
			if !ok {
				continue
			}
			go h.handleAPIEvent(apiEvent)

		case socketmode.EventTypeConnecting:
			log.Println("connecting to slack...")

		case socketmode.EventTypeConnected:
			log.Println("connected to slack via socket mode")
		}
	}
}

func (h *Handler) handleAPIEvent(event slackevents.EventsAPIEvent) {
	switch event.InnerEvent.Type {
	case "app_mention":
		ev, ok := event.InnerEvent.Data.(*slackevents.AppMentionEvent)
		if !ok {
			return
		}
		text := stripMention(ev.Text)
		source, question := h.parseSource(text)
		h.reply(ev.Channel, ev.TimeStamp, ev.User, source, question)

	case "message":
		ev, ok := event.InnerEvent.Data.(*slackevents.MessageEvent)
		if !ok {
			return
		}
		if ev.ChannelType == "im" && ev.BotID == "" {
			source, question := h.parseSource(ev.Text)
			h.reply(ev.Channel, ev.TimeStamp, ev.User, source, question)
		}
	}
}

func (h *Handler) reply(channel, threadTS, user, source, question string) {
	if strings.TrimSpace(question) == "" {
		return
	}

	log.Printf("┌─ New Request ────────────────────────────────")
	log.Printf("│ User    : %s", user)
	log.Printf("│ Channel : %s", channel)
	if source != "" {
		log.Printf("│ Source  : %s", source)
	}
	log.Printf("│ Question: %s", question)
	log.Printf("│ Thinking...")

	answer, err := h.askFn(context.Background(), source, question)
	if err != nil {
		log.Printf("│ ERROR: %v", err)
		log.Printf("└──────────────────────────────────────────────")
		answer = "Sorry, I ran into an error processing your question. Please try again."
	} else {
		log.Printf("│ Response Sent")
		log.Printf("└──────────────────────────────────────────────")
	}

	_, _, err = h.client.PostMessage(
		channel,
		slack.MsgOptionText(answer, false),
		slack.MsgOptionTS(threadTS),
	)
	if err != nil {
		log.Printf("slack post error: %v", err)
	}
}

// parseSource checks if the message starts with [source: <name>] and extracts it.
// Returns the source name (or "" if not specified) and the cleaned question.
func (h *Handler) parseSource(text string) (source, question string) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "[source:") {
		return "", text
	}

	end := strings.Index(text, "]")
	if end == -1 {
		return "", text
	}

	raw := strings.TrimPrefix(text[:end+1], "[source:")
	raw = strings.TrimSuffix(raw, "]")
	name := strings.TrimSpace(raw)

	// validate against known sources
	for _, s := range h.sourceNames {
		if strings.EqualFold(s, name) {
			return s, strings.TrimSpace(text[end+1:])
		}
	}

	// unknown source — tell the user
	known := strings.Join(h.sourceNames, ", ")
	return "", fmt.Sprintf("[source %q not found; available: %s] %s", name, known, strings.TrimSpace(text[end+1:]))
}

func stripMention(text string) string {
	if idx := strings.Index(text, ">"); idx != -1 {
		return strings.TrimSpace(text[idx+1:])
	}
	return strings.TrimSpace(text)
}
