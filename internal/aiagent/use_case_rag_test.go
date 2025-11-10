package aiagent

import (
	"context"
	"strings"
	"testing"
)

type keywordEmbedding struct{}

func (k *keywordEmbedding) vectorFor(text string) []float64 {
	lower := strings.ToLower(text)
	edge := 0.1
	web := 0.1
	if strings.Contains(lower, "edge") {
		edge = 1.0
	}
	if strings.Contains(lower, "web") {
		web = 1.0
	}
	return []float64{edge, web}
}

func (k *keywordEmbedding) GenerateEmbedding(ctx context.Context, text string) ([]float64, error) {
	return k.vectorFor(text), nil
}

func (k *keywordEmbedding) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float64, error) {
	out := make([][]float64, len(texts))
	for i, text := range texts {
		out[i] = k.vectorFor(text)
	}
	return out, nil
}

func TestUseCaseRAGFindsRelevantMatches(t *testing.T) {
	t.Parallel()

	config := &UseCasesConfig{UseCases: map[string]UseCaseConfig{
		"edge": {
			Name:              "edge",
			Description:       "Edge compute",
			EssentialPackages: []string{"systemd"},
		},
		"web": {
			Name:              "web",
			Description:       "Web server",
			EssentialPackages: []string{"nginx"},
		},
	}}

	rag, err := NewUseCaseRAG(context.Background(), config, &keywordEmbedding{})
	if err != nil {
		t.Fatalf("NewUseCaseRAG returned error: %v", err)
	}

	matches, err := rag.FindRelevantUseCases(context.Background(), "edge compute host", 2)
	if err != nil {
		t.Fatalf("FindRelevantUseCases returned error: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("expected at least one match")
	}
	if matches[0].Name != "edge" {
		t.Fatalf("expected top match to be 'edge', got %s", matches[0].Name)
	}
	if matches[0].Config == nil {
		t.Fatalf("expected match to include config reference")
	}
	if matches[0].ContextText == "" {
		t.Fatalf("expected contextual text for use case match")
	}
}
