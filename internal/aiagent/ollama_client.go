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

type OllamaConfig struct {
	BaseURL        string
	Model          string
	Temperature    float64
	NumPredict     int
	Timeout        int
	EmbeddingModel string
}

type OllamaChatModel struct {
	config  OllamaConfig
	client  *http.Client
	history []ChatMessage
}

type ollamaRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
	Options  ollamaOptions `json:"options,omitempty"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

type ollamaResponse struct {
	Model     string      `json:"model"`
	CreatedAt time.Time   `json:"created_at"`
	Message   ChatMessage `json:"message"`
	Done      bool        `json:"done"`
}

func NewOllamaChatModel(config OllamaConfig) *OllamaChatModel {
	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:11434"
	}
	if config.Model == "" {
		config.Model = "llama3.1:8b"
	}
	if config.Temperature == 0 {
		config.Temperature = 0.7
	}
	if config.NumPredict == 0 {
		config.NumPredict = 2000
	}
	if config.Timeout <= 0 {
		config.Timeout = 120
	}
	if config.EmbeddingModel == "" {
		config.EmbeddingModel = "nomic-embed-text"
	}

	return &OllamaChatModel{
		config:  config,
		client:  &http.Client{Timeout: time.Duration(config.Timeout) * time.Second},
		history: make([]ChatMessage, 0),
	}
}

func (ocm *OllamaChatModel) SendMessage(ctx context.Context, userMessage string) (string, error) {
	ocm.history = append(ocm.history, ChatMessage{
		Role:    "user",
		Content: userMessage,
	})

	request := ollamaRequest{
		Model:    ocm.config.Model,
		Messages: ocm.history,
		Stream:   false,
		Options: ollamaOptions{
			Temperature: ocm.config.Temperature,
			NumPredict:  ocm.config.NumPredict,
		},
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/chat", ocm.config.BaseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := ocm.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Ollama API error (status %d): %s", resp.StatusCode, string(body))
	}

	var ollamaResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	assistantMessage := ollamaResp.Message.Content
	ocm.history = append(ocm.history, ChatMessage{
		Role:    "assistant",
		Content: assistantMessage,
	})

	return assistantMessage, nil
}

func (ocm *OllamaChatModel) SendStructuredMessage(ctx context.Context, userMessage string, schema interface{}) error {
	response, err := ocm.SendMessage(ctx, userMessage)
	if err != nil {
		return err
	}

	jsonStr := extractJSON(response)
	return json.Unmarshal([]byte(jsonStr), schema)
}

func (ocm *OllamaChatModel) SetSystemPrompt(prompt string) {
	ocm.history = []ChatMessage{
		{
			Role:    "system",
			Content: prompt,
		},
	}
}

func (ocm *OllamaChatModel) ResetConversation() {
	ocm.history = make([]ChatMessage, 0)
}
