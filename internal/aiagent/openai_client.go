package aiagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenAIConfig for OpenAI API
type OpenAIConfig struct {
	APIKey      string
	Model       string
	Temperature float64
	MaxTokens   int
}

// OpenAIChatModel implements ChatModel interface for OpenAI
type OpenAIChatModel struct {
	config  OpenAIConfig
	client  *http.Client
	history []ChatMessage
}

// OpenAI API request/response structures
type openAIRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

type openAIResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int         `json:"index"`
		Message      ChatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type openAIError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// NewOpenAIChatModel creates a new OpenAI client
func NewOpenAIChatModel(config OpenAIConfig) *OpenAIChatModel {
	if config.Model == "" {
		config.Model = "gpt-4-turbo"
	}
	if config.Temperature == 0 {
		config.Temperature = 0.7
	}
	if config.MaxTokens == 0 {
		config.MaxTokens = 2000
	}

	return &OpenAIChatModel{
		config:  config,
		client:  &http.Client{Timeout: 120 * time.Second},
		history: make([]ChatMessage, 0),
	}
}

// SendMessage sends a message and gets response
func (oai *OpenAIChatModel) SendMessage(ctx context.Context, userMessage string) (string, error) {
	// Add user message to history
	oai.history = append(oai.history, ChatMessage{
		Role:    "user",
		Content: userMessage,
	})

	// Prepare request
	request := openAIRequest{
		Model:       oai.config.Model,
		Messages:    oai.history,
		Temperature: oai.config.Temperature,
		MaxTokens:   oai.config.MaxTokens,
	}

	// Marshal request
	requestBody, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.openai.com/v1/chat/completions",
		bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", oai.config.APIKey))

	// Send request
	resp, err := oai.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		var errResp openAIError
		if err := json.Unmarshal(body, &errResp); err == nil {
			return "", fmt.Errorf("OpenAI API error: %s", errResp.Error.Message)
		}
		return "", fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var openaiResp openAIResponse
	if err := json.Unmarshal(body, &openaiResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(openaiResp.Choices) == 0 {
		return "", fmt.Errorf("no response from OpenAI")
	}

	// Add assistant response to history
	assistantMessage := openaiResp.Choices[0].Message.Content
	oai.history = append(oai.history, ChatMessage{
		Role:    "assistant",
		Content: assistantMessage,
	})

	return assistantMessage, nil
}

// SendStructuredMessage expects JSON response
func (oai *OpenAIChatModel) SendStructuredMessage(ctx context.Context, userMessage string, schema interface{}) error {
	response, err := oai.SendMessage(ctx, userMessage)
	if err != nil {
		return err
	}

	// Extract JSON from response (GPT sometimes adds markdown)
	jsonStr := extractJSON(response)

	// Parse JSON response into provided schema
	return json.Unmarshal([]byte(jsonStr), schema)
}

// SetSystemPrompt sets the initial system instruction
func (oai *OpenAIChatModel) SetSystemPrompt(prompt string) {
	oai.history = []ChatMessage{
		{
			Role:    "system",
			Content: prompt,
		},
	}
}

// ResetConversation clears history
func (oai *OpenAIChatModel) ResetConversation() {
	oai.history = make([]ChatMessage, 0)
}
