package aiagent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type capturingEmbeddingClient struct {
	batchCalls int
	lastTexts  []string
	embedVec   []float64
}

func (c *capturingEmbeddingClient) GenerateEmbedding(ctx context.Context, text string) ([]float64, error) {
	if len(c.embedVec) == 0 {
		return []float64{1, 0, 0}, nil
	}
	return append([]float64(nil), c.embedVec...), nil
}

func (c *capturingEmbeddingClient) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float64, error) {
	c.batchCalls++
	c.lastTexts = append([]string(nil), texts...)

	embeddings := make([][]float64, len(texts))
	for i := range texts {
		embeddings[i] = []float64{float64(i + 1), float64(i), 0}
	}
	if len(embeddings) == 0 {
		embeddings = [][]float64{{1, 0, 0}}
	}
	return embeddings, nil
}

func TestNewTemplateRAGIndexesTemplates(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	templateYAML := `image:
  name: edge
  version: "1.0"
target:
  dist: azl3
  arch: x86_64
  imageType: raw
systemConfig:
  name: edge
  description: Edge template
  packages:
    - nginx
`
	if err := os.WriteFile(filepath.Join(dir, "edge.yml"), []byte(templateYAML), 0600); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	emb := &capturingEmbeddingClient{}
	rag, err := NewTemplateRAG(dir, emb)
	if err != nil {
		t.Fatalf("NewTemplateRAG returned error: %v", err)
	}

	if len(rag.templates) != 1 {
		t.Fatalf("expected 1 template, got %d", len(rag.templates))
	}
	if len(rag.templatesByUse["edge"]) != 1 {
		t.Fatalf("expected edge use case to be indexed")
	}
	if emb.batchCalls == 0 {
		t.Fatalf("expected embedding generation to be invoked")
	}
	if len(emb.lastTexts) != 1 {
		t.Fatalf("expected embedding client to receive template text, got %d entries", len(emb.lastTexts))
	}
}

type staticVectorEmbedding struct {
	query []float64
}

func (s *staticVectorEmbedding) GenerateEmbedding(ctx context.Context, text string) ([]float64, error) {
	return append([]float64(nil), s.query...), nil
}

func (s *staticVectorEmbedding) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float64, error) {
	out := make([][]float64, len(texts))
	for i := range texts {
		out[i] = append([]float64(nil), s.query...)
	}
	return out, nil
}

func TestFindRelevantTemplatesReturnsSortedResults(t *testing.T) {
	t.Parallel()

	rag := &TemplateRAG{
		templates: map[string]*TemplateExample{
			"high": {Name: "high", Embedding: []float64{1, 0}, UseCase: "edge"},
			"low":  {Name: "low", Embedding: []float64{0, 1}, UseCase: "edge"},
		},
		embeddingClient: &staticVectorEmbedding{query: []float64{1, 0}},
	}

	results, err := rag.FindRelevantTemplates(context.Background(), "edge", 2)
	if err != nil {
		t.Fatalf("FindRelevantTemplates returned error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Template.Name != "high" || results[0].Score <= results[1].Score {
		t.Fatalf("expected results sorted by score descending, got %#v", results)
	}
}

func TestBuildSearchableTextIncludesMetadata(t *testing.T) {
	t.Parallel()

	rag := &TemplateRAG{}
	text := rag.buildSearchableText(&TemplateExample{
		UseCase:      "edge",
		Description:  "Edge device",
		Distribution: "azl3",
		Architecture: "x86_64",
		ImageType:    "raw",
		Packages:     []string{"nginx", "openssl"},
		HasDisk:      true,
		HasKernel:    true,
		Repositories: []string{"custom"},
	})

	for _, expected := range []string{"Use case: edge", "Packages: nginx", "Repositories: custom"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected searchable text to contain %q, got %q", expected, text)
		}
	}
}

func TestCosineSimilarityHandlesVectors(t *testing.T) {
	t.Parallel()

	if score := cosineSimilarity([]float64{1, 0}, []float64{0, 1}); score != 0 {
		t.Fatalf("expected orthogonal vectors to have similarity 0, got %f", score)
	}
	if score := cosineSimilarity([]float64{2, 2}, []float64{1, 1}); score < 0.99 {
		t.Fatalf("expected parallel vectors to be close to 1, got %f", score)
	}
	if score := cosineSimilarity([]float64{1, 2}, []float64{1}); score != 0 {
		t.Fatalf("expected mismatched dimensions to yield 0, got %f", score)
	}
}

func TestInferUseCaseHeuristics(t *testing.T) {
	t.Parallel()

	if use := inferUseCaseFromName("High performance web server"); use != "web-server" {
		t.Fatalf("expected web-server use case, got %q", use)
	}
	if use := inferUseCaseFromFilename("sample-minimal.yml"); use != "minimal" {
		t.Fatalf("expected minimal use case from filename, got %q", use)
	}
}

func TestTemplateRAGHelpers(t *testing.T) {
	t.Parallel()

	example := &TemplateExample{
		Name:         "example.yml",
		UseCase:      "edge",
		Description:  "Edge template",
		Distribution: "azl3",
		Architecture: "x86_64",
		ImageType:    "raw",
		Packages:     []string{"nginx", "openssl", "curl"},
		Repositories: []string{"repo1"},
	}

	rag := &TemplateRAG{
		templatesByUse: map[string][]*TemplateExample{
			"edge": {example},
			"web":  {},
		},
	}

	prompt := rag.FormatTemplateForPrompt(example, false)
	if !strings.Contains(prompt, "Template: example.yml") {
		t.Fatalf("expected prompt header, got %q", prompt)
	}

	if len(rag.GetTemplatesByUseCase("edge")) != 1 {
		t.Fatalf("expected one template for edge")
	}

	useCases := rag.GetAllUseCases()
	if len(useCases) != 2 || useCases[0] != "edge" {
		t.Fatalf("expected sorted use cases, got %v", useCases)
	}
}

func TestGenerateEmbeddingsAssignsVectors(t *testing.T) {
	t.Parallel()

	rag := &TemplateRAG{embeddingClient: &capturingEmbeddingClient{embedVec: []float64{0.5, 0.25}}}
	templates := []*TemplateExample{
		{Name: "first"},
		{Name: "second"},
	}

	if err := rag.generateEmbeddings(context.Background(), templates); err != nil {
		t.Fatalf("generateEmbeddings returned error: %v", err)
	}

	for _, tpl := range templates {
		if len(tpl.Embedding) == 0 {
			t.Fatalf("expected template %s to receive embedding", tpl.Name)
		}
	}
}
