package claude

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const (
	defaultMaxTokens = 4096
)

type Client struct {
	client anthropic.Client
}

type Message struct {
	Role    string
	Content string
}

func NewClient(apiKey string) *Client {
	return &Client{
		client: anthropic.NewClient(
			option.WithAPIKey(apiKey),
		),
	}
}

func (c *Client) SendMessage(prompt string, systemPrompt string) (string, error) {
	params := anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeSonnet4_20250514,
		MaxTokens: defaultMaxTokens,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	}

	if systemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: systemPrompt},
		}
	}

	msg, err := c.client.Messages.New(context.Background(), params)
	if err != nil {
		return "", fmt.Errorf("send message: %w", err)
	}

	if len(msg.Content) == 0 {
		return "", fmt.Errorf("empty response from API")
	}

	return msg.Content[0].Text, nil
}

func (c *Client) SendConversation(messages []Message, systemPrompt string) (string, error) {
	msgParams := make([]anthropic.MessageParam, len(messages))
	for i, msg := range messages {
		if msg.Role == "user" {
			msgParams[i] = anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content))
		} else {
			msgParams[i] = anthropic.NewAssistantMessage(anthropic.NewTextBlock(msg.Content))
		}
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeSonnet4_20250514,
		MaxTokens: defaultMaxTokens,
		Messages:  msgParams,
	}

	if systemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: systemPrompt},
		}
	}

	msg, err := c.client.Messages.New(context.Background(), params)
	if err != nil {
		return "", fmt.Errorf("send conversation: %w", err)
	}

	if len(msg.Content) == 0 {
		return "", fmt.Errorf("empty response from API")
	}

	return msg.Content[0].Text, nil
}
