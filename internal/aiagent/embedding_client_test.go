package aiagent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestNewOllamaEmbeddingClientDefaults(t *testing.T) {
	t.Parallel()

	client := NewOllamaEmbeddingClient("", "", 0)
	if client.model != "nomic-embed-text" {
		t.Fatalf("expected default embedding model, got %q", client.model)
	}
	if client.client.Timeout != 120*time.Second {
		t.Fatalf("expected default timeout, got %v", client.client.Timeout)
	}
}

func TestOllamaEmbeddingClientGeneratesEmbeddings(t *testing.T) {
	t.Parallel()

	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/embeddings" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"embedding":[0.1,0.2]}`)
	}))
	defer server.Close()

	client := NewOllamaEmbeddingClient(server.URL, "custom", 5)

	vec, err := client.GenerateEmbedding(context.Background(), "hello")
	if err != nil {
		t.Fatalf("GenerateEmbedding returned error: %v", err)
	}
	if len(vec) != 2 || vec[0] != 0.1 {
		t.Fatalf("unexpected embedding vector %v", vec)
	}

	batch, err := client.GenerateBatchEmbeddings(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatalf("GenerateBatchEmbeddings returned error: %v", err)
	}
	if len(batch) != 2 {
		t.Fatalf("expected 2 embeddings in batch, got %d", len(batch))
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls to server (1 + batch of 2), got %d", calls)
	}
}

func TestOpenAIEmbeddingClientDefaults(t *testing.T) {
	t.Parallel()

	client := NewOpenAIEmbeddingClient("key", "", 0)
	if client.model != "text-embedding-3-small" {
		t.Fatalf("expected default OpenAI embedding model, got %q", client.model)
	}
	if client.client.Timeout != 60*time.Second {
		t.Fatalf("expected default timeout of 60s, got %v", client.client.Timeout)
	}
}

func TestOpenAIEmbeddingClientGeneratesEmbeddings(t *testing.T) {
	t.Parallel()

	client := NewOpenAIEmbeddingClient("key", "model", 5)
	client.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://api.openai.com/v1/embeddings" {
			t.Fatalf("unexpected OpenAI URL %s", req.URL.String())
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		req.Body.Close()

		var parsed openaiEmbeddingRequest
		if err := json.Unmarshal(body, &parsed); err != nil {
			t.Fatalf("failed to unmarshal request body: %v", err)
		}

		respPayload := openaiEmbeddingResponse{Data: make([]struct {
			Embedding []float64 `json:"embedding"`
			Index     int       `json:"index"`
		}, len(parsed.Input))}

		for i := range respPayload.Data {
			respPayload.Data[i].Index = i
			respPayload.Data[i].Embedding = []float64{float64(i), float64(i + 1)}
		}

		data, err := json.Marshal(respPayload)
		if err != nil {
			t.Fatalf("failed to marshal response: %v", err)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(string(data))),
			Header:     make(http.Header),
		}, nil
	})

	vec, err := client.GenerateEmbedding(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("GenerateEmbedding returned error: %v", err)
	}
	if len(vec) != 2 || vec[0] != 0 {
		t.Fatalf("unexpected embedding vector %v", vec)
	}

	batch, err := client.GenerateBatchEmbeddings(context.Background(), []string{"first", "second"})
	if err != nil {
		t.Fatalf("GenerateBatchEmbeddings returned error: %v", err)
	}
	if len(batch) != 2 || batch[1][0] != 1 {
		t.Fatalf("unexpected batch embeddings %v", batch)
	}
}

func TestOpenAIEmbeddingClientErrorHandling(t *testing.T) {
	t.Parallel()

	client := NewOpenAIEmbeddingClient("key", "model", 5)
	client.client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadGateway,
			Body:       io.NopCloser(strings.NewReader("bad gateway")),
			Header:     make(http.Header),
		}, nil
	})

	if _, err := client.GenerateEmbedding(context.Background(), "prompt"); err == nil {
		t.Fatalf("expected error when OpenAI API returns non-200 status")
	}
}
