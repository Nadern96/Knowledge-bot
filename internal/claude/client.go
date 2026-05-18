package claude

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

const systemPrompt = `You are a helpful knowledge assistant for an engineering team.
Your job is to answer questions based strictly on the wiki documentation provided below.
- If the answer is clearly in the wiki, answer it directly and concisely.
- If the answer is partially covered, share what you know and note the gaps.
- If the answer is not in the wiki at all, say so clearly — do not make things up.
- Use bullet points or short paragraphs for readability.
- Reference the page name when citing specific information (e.g. "According to the Service-Auth page...").

Wiki documentation:
%s`

type Client struct {
	api   *genai.Client
	model string
}

func NewClient(apiKey string) (*Client, error) {
	ctx := context.Background()
	api, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, err
	}
	return &Client{
		api:   api,
		model: "gemini-3.1-flash-lite",
	}, nil
}

func (c *Client) Ask(ctx context.Context, wikiContext, question string) (string, error) {
	prompt := fmt.Sprintf(systemPrompt, wikiContext) + "\n\nQuestion: " + question

	result, err := c.api.Models.GenerateContent(ctx, c.model,
		genai.Text(prompt),
		nil,
	)
	if err != nil {
		return "", err
	}

	if result == nil || len(result.Candidates) == 0 {
		return "", fmt.Errorf("empty response from gemini")
	}

	return result.Text(), nil
}
