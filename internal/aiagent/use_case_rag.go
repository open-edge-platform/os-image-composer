package aiagent

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// UseCaseMatch represents a curated use case and its similarity score
type UseCaseMatch struct {
	Name        string
	Score       float64
	Config      *UseCaseConfig
	ContextText string
}

// UseCaseRAG provides semantic search over curated use case definitions
type UseCaseRAG struct {
	entries         map[string]*useCaseEntry
	useCases        *UseCasesConfig
	embeddingClient EmbeddingGenerator
}

type useCaseEntry struct {
	Name      string
	Config    UseCaseConfig
	Embedding []float64
	Context   string
}

// NewUseCaseRAG builds an embedding index for curated use cases
func NewUseCaseRAG(ctx context.Context, config *UseCasesConfig, embeddingClient EmbeddingGenerator) (*UseCaseRAG, error) {
	if config == nil {
		return nil, fmt.Errorf("use case config is required")
	}
	if embeddingClient == nil {
		return nil, fmt.Errorf("embedding client is required")
	}

	rag := &UseCaseRAG{
		entries:         make(map[string]*useCaseEntry, len(config.UseCases)),
		useCases:        config,
		embeddingClient: embeddingClient,
	}

	if err := rag.indexUseCases(ctx); err != nil {
		return nil, err
	}

	return rag, nil
}

func (rag *UseCaseRAG) indexUseCases(ctx context.Context) error {
	if len(rag.useCases.UseCases) == 0 {
		return fmt.Errorf("no use cases defined")
	}

	texts := make([]string, 0, len(rag.useCases.UseCases))
	names := make([]string, 0, len(rag.useCases.UseCases))

	for name, cfg := range rag.useCases.UseCases {
		entry := &useCaseEntry{
			Name:    name,
			Config:  cfg,
			Context: buildUseCaseContextText(name, cfg),
		}
		rag.entries[name] = entry
		names = append(names, name)
		texts = append(texts, entry.Context)
	}

	embeddings, err := rag.embeddingClient.GenerateBatchEmbeddings(ctx, texts)
	if err != nil {
		return fmt.Errorf("failed to generate use case embeddings: %w", err)
	}

	for i, name := range names {
		rag.entries[name].Embedding = embeddings[i]
	}

	return nil
}

// FindRelevantUseCases returns top matches for the user query
func (rag *UseCaseRAG) FindRelevantUseCases(ctx context.Context, query string, topK int) ([]*UseCaseMatch, error) {
	if rag == nil {
		return nil, fmt.Errorf("use case RAG not initialized")
	}

	queryEmbedding, err := rag.embeddingClient.GenerateEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate use case query embedding: %w", err)
	}

	matches := make([]*UseCaseMatch, 0, len(rag.entries))
	for _, entry := range rag.entries {
		if len(entry.Embedding) == 0 {
			continue
		}
		score := cosineSimilarity(queryEmbedding, entry.Embedding)
		sanitizedScore := score
		if sanitizedScore < 0 {
			sanitizedScore = 0
		}
		matches = append(matches, &UseCaseMatch{
			Name:        entry.Name,
			Score:       sanitizedScore,
			Config:      &entry.Config,
			ContextText: entry.Context,
		})
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Score > matches[j].Score
	})

	if topK <= 0 || topK > len(matches) {
		topK = len(matches)
	}

	return matches[:topK], nil
}

// buildUseCaseContextText creates descriptive text for embeddings and prompts
func buildUseCaseContextText(name string, cfg UseCaseConfig) string {
	sections := []string{
		fmt.Sprintf("Use case: %s", name),
	}
	if cfg.Description != "" {
		sections = append(sections, fmt.Sprintf("Description: %s", cfg.Description))
	}
	if len(cfg.Keywords) > 0 {
		sections = append(sections, fmt.Sprintf("Keywords: %s", strings.Join(cfg.Keywords, ", ")))
	}
	if len(cfg.EssentialPackages) > 0 {
		sections = append(sections, fmt.Sprintf("Essential packages: %s", strings.Join(cfg.EssentialPackages, ", ")))
	}
	if len(cfg.OptionalPackages) > 0 {
		sections = append(sections, fmt.Sprintf("Optional packages: %s", strings.Join(cfg.OptionalPackages, ", ")))
	}
	if len(cfg.SecurityPackages) > 0 {
		sections = append(sections, fmt.Sprintf("Security packages: %s", strings.Join(cfg.SecurityPackages, ", ")))
	}
	if len(cfg.PerformancePackages) > 0 {
		sections = append(sections, fmt.Sprintf("Performance packages: %s", strings.Join(cfg.PerformancePackages, ", ")))
	}
	if cfg.Kernel.DefaultVersion != "" {
		sections = append(sections, fmt.Sprintf("Kernel default: %s", cfg.Kernel.DefaultVersion))
	}
	if cfg.Kernel.Cmdline != "" {
		sections = append(sections, fmt.Sprintf("Kernel cmdline: %s", cfg.Kernel.Cmdline))
	}
	if cfg.Disk.DefaultSize != "" {
		sections = append(sections, fmt.Sprintf("Disk size: %s", cfg.Disk.DefaultSize))
	}

	return strings.Join(sections, ". ")
}

// UseCaseNames returns the curated use case keys in deterministic order
func (rag *UseCaseRAG) UseCaseNames() []string {
	if rag == nil {
		return nil
	}

	names := make([]string, 0, len(rag.entries))
	for name := range rag.entries {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
