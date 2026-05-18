package slack

import (
	"context"
	"log"
	"strings"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// AskFunc is the function the handler calls to get an answer from Claude.
type AskFunc func(ctx context.Context, question string) (string, error)

type Handler struct {
	client *slack.Client
	socket *socketmode.Client
	askFn  AskFunc
}

func NewHandler(botToken, appToken string, askFn AskFunc) *Handler {
	client := slack.New(
		botToken,
		slack.OptionAppLevelToken(appToken),
	)
	socket := socketmode.New(client)
	return &Handler{
		client: client,
		socket: socket,
		askFn:  askFn,
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
		question := stripMention(ev.Text)
		h.reply(ev.Channel, ev.TimeStamp, ev.User, question)

	case "message":
		ev, ok := event.InnerEvent.Data.(*slackevents.MessageEvent)
		if !ok {
			return
		}
		if ev.ChannelType == "im" && ev.BotID == "" {
			h.reply(ev.Channel, ev.TimeStamp, ev.User, ev.Text)
		}
	}
}

func (h *Handler) reply(channel, threadTS, user, question string) {
	if strings.TrimSpace(question) == "" {
		return
	}

	log.Printf("┌─ New Request ────────────────────────────────")
	log.Printf("│ User    : %s", user)
	log.Printf("│ Channel : %s", channel)
	log.Printf("│ Question: %s", question)
	log.Printf("│ Thinking...")

	answer, err := h.askFn(context.Background(), question)
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

func stripMention(text string) string {
	// Slack mentions look like <@U12345678>
	if idx := strings.Index(text, ">"); idx != -1 {
		return strings.TrimSpace(text[idx+1:])
	}
	return strings.TrimSpace(text)
}
