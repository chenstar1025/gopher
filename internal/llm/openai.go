package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// OpenAIClient implements LLM via any OpenAI-compatible API.
type OpenAIClient struct {
	client  *http.Client
	baseURL string
	apiKey  string
	model   string
}

// NewOpenAI creates an OpenAI-compatible LLM client.
func NewOpenAI(baseURL, apiKey, model string) *OpenAIClient {
	return &OpenAIClient{
		client:  &http.Client{},
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
	}
}

type chatRequest struct {
	Model    string          `json:"model"`
	Messages []chatMessage   `json:"messages"`
	Tools    []chatTool      `json:"tools,omitempty"`
	Stream   bool            `json:"stream"`
}

type chatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []chatToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type chatTool struct {
	Type     string       `json:"type"`
	Function toolFunction `json:"function"`
}

type toolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type chatToolCall struct {
	ID       string        `json:"id"`
	Type     string        `json:"type"`
	Function toolCallFunc  `json:"function"`
}

type toolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
	Error   *chatError   `json:"error,omitempty"`
}

type chatChoice struct {
	Message      chatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type chatError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// Chat sends a request to the OpenAI-compatible API and parses the response.
func (c *OpenAIClient) Chat(ctx context.Context, messages []Message, tools []ToolDef) (*Response, error) {
	req := chatRequest{
		Model:    c.model,
		Messages: toChatMessages(messages),
		Tools:    toChatTools(tools),
		Stream:   false,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	// Read and buffer the response body for potential error parsing
	var buf bytes.Buffer
	respBody, err := io.ReadAll(io.TeeReader(resp.Body, &buf))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Try to extract error from response body
		var chatResp chatResponse
		if json.Unmarshal(respBody, &chatResp) == nil && chatResp.Error != nil {
			return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, chatResp.Error.Message)
		}
		// Fallback: read first line of body
		scanner := bufio.NewScanner(&buf)
		firstLine := ""
		if scanner.Scan() {
			firstLine = scanner.Text()
		}
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, firstLine)
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := chatResp.Choices[0]
	result := &Response{
		Messages:     []Message{fromChatMessage(choice.Message)},
		FinishReason: choice.FinishReason,
	}

	for _, tc := range choice.Message.ToolCalls {
		args := map[string]any{}
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			args = map[string]any{"raw": tc.Function.Arguments}
		}
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			ID:   tc.ID,
			Name: tc.Function.Name,
			Args: args,
		})
	}

	return result, nil
}

func toChatMessages(msgs []Message) []chatMessage {
	out := make([]chatMessage, len(msgs))
	for i, m := range msgs {
		cm := chatMessage{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
		}
		for _, tc := range m.ToolCalls {
			argsJSON, _ := json.Marshal(tc.Args)
			cm.ToolCalls = append(cm.ToolCalls, chatToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: toolCallFunc{
					Name:      tc.Name,
					Arguments: string(argsJSON),
				},
			})
		}
		out[i] = cm
	}
	return out
}

func toChatTools(tools []ToolDef) []chatTool {
	out := make([]chatTool, len(tools))
	for i, t := range tools {
		out[i] = chatTool{
			Type: "function",
			Function: toolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		}
	}
	return out
}

func fromChatMessage(cm chatMessage) Message {
	return Message{
		Role:    cm.Role,
		Content: cm.Content,
	}
}
