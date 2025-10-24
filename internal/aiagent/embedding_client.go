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

// OllamaEmbeddingClient generates embeddings using Ollama's embedding API
type OllamaEmbeddingClient struct {
	baseURL string
	model   string
	client  *http.Client
}

// NewOllamaEmbeddingClient creates a new Ollama embedding client
func NewOllamaEmbeddingClient(baseURL, model string) *OllamaEmbeddingClient {
	if model == "" {
		// Use smaller embedding model for efficiency
		model = "nomic-embed-text"
	}

	return &OllamaEmbeddingClient{
		baseURL: baseURL,
		model:   model,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

type ollamaEmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type ollamaEmbeddingResponse struct {
	Embedding []float64 `json:"embedding"`
}

// GenerateEmbedding generates a single embedding vector
func (c *OllamaEmbeddingClient) GenerateEmbedding(ctx context.Context, text string) ([]float64, error) {
	reqBody := ollamaEmbeddingRequest{
		Model:  c.model,
		Prompt: text,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/embeddings", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama API error (status %d): %s", resp.StatusCode, string(body))
	}

	var embResp ollamaEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return embResp.Embedding, nil
}

// GenerateBatchEmbeddings generates embeddings for multiple texts
func (c *OllamaEmbeddingClient) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float64, error) {
	embeddings := make([][]float64, len(texts))

	for i, text := range texts {
		embedding, err := c.GenerateEmbedding(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("failed to generate embedding %d: %w", i, err)
		}
		embeddings[i] = embedding

		// Show progress
		if (i+1)%5 == 0 || i == len(texts)-1 {
			fmt.Printf("   Generated %d/%d embeddings\n", i+1, len(texts))
		}
	}

	return embeddings, nil
}

// OpenAIEmbeddingClient generates embeddings using OpenAI's API
type OpenAIEmbeddingClient struct {
	apiKey string
	model  string
	client *http.Client
}

// NewOpenAIEmbeddingClient creates a new OpenAI embedding client
func NewOpenAIEmbeddingClient(apiKey, model string) *OpenAIEmbeddingClient {
	if model == "" {
		model = "text-embedding-3-small" // Cost-effective embedding model
	}

	return &OpenAIEmbeddingClient{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

type openaiEmbeddingRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

type openaiEmbeddingResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
}

// GenerateEmbedding generates a single embedding vector
func (c *OpenAIEmbeddingClient) GenerateEmbedding(ctx context.Context, text string) ([]float64, error) {
	embeddings, err := c.GenerateBatchEmbeddings(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return embeddings[0], nil
}

// GenerateBatchEmbeddings generates embeddings for multiple texts
func (c *OpenAIEmbeddingClient) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float64, error) {
	// OpenAI supports batch requests (up to ~2048 texts)
	const batchSize = 100

	allEmbeddings := make([][]float64, len(texts))

	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}

		batch := texts[i:end]
		batchEmbeddings, err := c.generateBatch(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("failed to generate batch %d-%d: %w", i, end, err)
		}

		copy(allEmbeddings[i:], batchEmbeddings)

		fmt.Printf("   Generated %d/%d embeddings\n", end, len(texts))
	}

	return allEmbeddings, nil
}

func (c *OpenAIEmbeddingClient) generateBatch(ctx context.Context, texts []string) ([][]float64, error) {
	reqBody := openaiEmbeddingRequest{
		Input: texts,
		Model: c.model,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.openai.com/v1/embeddings", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}

	var embResp openaiEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract embeddings in correct order
	embeddings := make([][]float64, len(texts))
	for _, data := range embResp.Data {
		embeddings[data.Index] = data.Embedding
	}

	return embeddings, nil
}
