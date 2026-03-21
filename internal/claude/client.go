package claude

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type Client struct {
	backendURL string
	httpClient *http.Client
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Messages   []Message `json:"messages"`
	CurrentSQL string    `json:"current_sql"`
	Tables     []string  `json:"tables"`
	LastError  string    `json:"last_error"`
}

type DiffPayload struct {
	OldSQL string `json:"old_sql"`
	NewSQL string `json:"new_sql"`
}

type DonePayload struct {
	FullResponse string      `json:"full_response"`
	Diff         DiffPayload `json:"diff"`
}

type StreamEvent struct {
	Type    string // "token", "done", "error"
	Content string // token text
	Done    *DonePayload
	Error   string
}

func NewClient(backendURL string) *Client {
	if backendURL == "" {
		backendURL = "http://localhost:8080"
	}
	return &Client{
		backendURL: strings.TrimRight(backendURL, "/"),
		httpClient: &http.Client{},
	}
}

func (c *Client) SendMessage(prompt string, systemPrompt string) (string, error) {
	req := ChatRequest{
		Messages: []Message{{Role: "user", Content: prompt}},
	}

	events := c.stream(req)
	var response string
	for event := range events {
		switch event.Type {
		case "done":
			if event.Done != nil {
				response = event.Done.FullResponse
			}
		case "error":
			return "", fmt.Errorf("%s", event.Error)
		}
	}
	return response, nil
}

func (c *Client) SendConversation(messages []Message, systemPrompt string) (string, error) {
	req := ChatRequest{
		Messages: messages,
	}

	events := c.stream(req)
	var response string
	for event := range events {
		switch event.Type {
		case "done":
			if event.Done != nil {
				response = event.Done.FullResponse
			}
		case "error":
			return "", fmt.Errorf("%s", event.Error)
		}
	}
	return response, nil
}

func (c *Client) SendMessageStream(req ChatRequest) <-chan StreamEvent {
	return c.stream(req)
}

func (c *Client) stream(req ChatRequest) <-chan StreamEvent {
	events := make(chan StreamEvent)

	go func() {
		defer close(events)

		body, err := json.Marshal(req)
		if err != nil {
			events <- StreamEvent{Type: "error", Error: fmt.Sprintf("marshal error: %v", err)}
			return
		}

		httpReq, err := http.NewRequest("POST", c.backendURL+"/chat", bytes.NewReader(body))
		if err != nil {
			events <- StreamEvent{Type: "error", Error: fmt.Sprintf("request error: %v", err)}
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Accept", "text/event-stream")

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			events <- StreamEvent{Type: "error", Error: fmt.Sprintf("connection error: %v", err)}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			events <- StreamEvent{Type: "error", Error: fmt.Sprintf("server error: %s", resp.Status)}
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		var eventType string

		for scanner.Scan() {
			line := scanner.Text()

			if strings.HasPrefix(line, "event: ") {
				eventType = strings.TrimPrefix(line, "event: ")
				continue
			}

			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")

				switch eventType {
				case "token":
					var payload struct {
						Content string `json:"content"`
					}
					if json.Unmarshal([]byte(data), &payload) == nil {
						events <- StreamEvent{Type: "token", Content: payload.Content}
					}

				case "done":
					var payload DonePayload
					if json.Unmarshal([]byte(data), &payload) == nil {
						events <- StreamEvent{Type: "done", Done: &payload}
					}

				case "error":
					var payload struct {
						Error string `json:"error"`
					}
					if json.Unmarshal([]byte(data), &payload) == nil {
						events <- StreamEvent{Type: "error", Error: payload.Error}
					}
				}

				eventType = ""
			}
		}
	}()

	return events
}
