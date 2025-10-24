package aiagent

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// TemplateExample represents a parsed template with metadata
type TemplateExample struct {
	Name         string
	FilePath     string
	Content      string
	Description  string
	UseCase      string
	Distribution string
	Architecture string
	ImageType    string
	Packages     []string
	HasDisk      bool
	HasKernel    bool
	KernelInfo   string
	Repositories []string
	Embedding    []float64 // Pre-computed embedding vector
}

// TemplateRAG handles retrieval-augmented generation using template examples
type TemplateRAG struct {
	templates       map[string]*TemplateExample
	templatesByUse  map[string][]*TemplateExample
	embeddingClient EmbeddingGenerator
}

// EmbeddingGenerator interface for different embedding providers
type EmbeddingGenerator interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float64, error)
	GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float64, error)
}

// SearchResult represents a template search result with relevance score
type SearchResult struct {
	Template *TemplateExample
	Score    float64
}

// NewTemplateRAG creates a new RAG system and indexes all templates
func NewTemplateRAG(templatesDir string, embeddingClient EmbeddingGenerator) (*TemplateRAG, error) {
	rag := &TemplateRAG{
		templates:       make(map[string]*TemplateExample),
		templatesByUse:  make(map[string][]*TemplateExample),
		embeddingClient: embeddingClient,
	}

	if err := rag.IndexTemplates(templatesDir); err != nil {
		return nil, fmt.Errorf("failed to index templates: %w", err)
	}

	return rag, nil
}

// IndexTemplates scans directory and indexes all template files
func (rag *TemplateRAG) IndexTemplates(templatesDir string) error {
	// Find all .yml files in templates directory
	files, err := filepath.Glob(filepath.Join(templatesDir, "*.yml"))
	if err != nil {
		return fmt.Errorf("failed to find template files: %w", err)
	}

	if len(files) == 0 {
		// Fallback to project root if templatesDir doesn't exist
		files, err = filepath.Glob("*.yml")
		if err != nil {
			return fmt.Errorf("failed to find template files in fallback location: %w", err)
		}
	}

	fmt.Printf("📚 Indexing %d template files...\n", len(files))

	// Filter and parse templates
	var validTemplates []*TemplateExample
	for _, file := range files {
		// Skip config files
		if strings.Contains(file, "use-cases.yml") ||
			strings.Contains(file, "os-image-composer.yml") ||
			strings.Contains(file, "config.yml") {
			continue
		}

		template, err := rag.parseTemplate(file)
		if err != nil {
			fmt.Printf("Warning: Failed to parse %s: %v\n", file, err)
			continue
		}

		validTemplates = append(validTemplates, template)
		rag.templates[template.Name] = template

		// Group by use case
		if template.UseCase != "" {
			rag.templatesByUse[template.UseCase] = append(rag.templatesByUse[template.UseCase], template)
		}
	}

	if len(validTemplates) == 0 {
		return fmt.Errorf("no valid templates found")
	}

	// Generate embeddings for all templates
	if err := rag.generateEmbeddings(context.Background(), validTemplates); err != nil {
		return fmt.Errorf("failed to generate embeddings: %w", err)
	}

	fmt.Printf("✅ Indexed %d templates across %d use cases\n", len(validTemplates), len(rag.templatesByUse))
	return nil
}

// parseTemplate reads and parses a template file
func (rag *TemplateRAG) parseTemplate(filePath string) (*TemplateExample, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	// Parse YAML to extract metadata
	var templateData map[string]interface{}
	if err := yaml.Unmarshal(content, &templateData); err != nil {
		return nil, err
	}

	template := &TemplateExample{
		Name:     filepath.Base(filePath),
		FilePath: filePath,
		Content:  string(content),
	}

	// Extract metadata from template structure
	if target, ok := templateData["target"].(map[string]interface{}); ok {
		if dist, ok := target["dist"].(string); ok {
			template.Distribution = dist
		}
		if arch, ok := target["arch"].(string); ok {
			template.Architecture = arch
		}
		if imgType, ok := target["imageType"].(string); ok {
			template.ImageType = imgType
		}
	}

	if sysConfig, ok := templateData["systemConfig"].(map[string]interface{}); ok {
		if desc, ok := sysConfig["description"].(string); ok {
			template.Description = desc
		}
		if name, ok := sysConfig["name"].(string); ok {
			template.UseCase = inferUseCaseFromName(name)
		}

		// Extract packages
		if pkgs, ok := sysConfig["packages"].([]interface{}); ok {
			for _, pkg := range pkgs {
				if pkgStr, ok := pkg.(string); ok {
					template.Packages = append(template.Packages, pkgStr)
				}
			}
		}

		// Extract kernel info
		if kernel, ok := sysConfig["kernel"].(map[string]interface{}); ok {
			template.HasKernel = true
			if ver, ok := kernel["version"].(string); ok {
				template.KernelInfo = ver
			}
		}
	}

	// Check for disk configuration
	if _, ok := templateData["disk"]; ok {
		template.HasDisk = true
	}

	// Extract package repositories
	if repos, ok := templateData["packageRepositories"].([]interface{}); ok {
		for _, repo := range repos {
			if repoMap, ok := repo.(map[string]interface{}); ok {
				if codename, ok := repoMap["codename"].(string); ok {
					template.Repositories = append(template.Repositories, codename)
				}
			}
		}
	}

	// Infer use case from filename if not found
	if template.UseCase == "" {
		template.UseCase = inferUseCaseFromFilename(template.Name)
	}

	return template, nil
}

// generateEmbeddings creates vector embeddings for all templates
func (rag *TemplateRAG) generateEmbeddings(ctx context.Context, templates []*TemplateExample) error {
	// Build searchable text for each template
	texts := make([]string, len(templates))
	for i, template := range templates {
		texts[i] = rag.buildSearchableText(template)
	}

	fmt.Printf("🔮 Generating embeddings for %d templates...\n", len(texts))

	// Generate embeddings in batch
	embeddings, err := rag.embeddingClient.GenerateBatchEmbeddings(ctx, texts)
	if err != nil {
		return err
	}

	// Store embeddings in templates
	for i, embedding := range embeddings {
		templates[i].Embedding = embedding
	}

	return nil
}

// buildSearchableText creates a rich text representation for embedding
func (rag *TemplateRAG) buildSearchableText(template *TemplateExample) string {
	parts := []string{
		// Use case and description
		fmt.Sprintf("Use case: %s", template.UseCase),
		fmt.Sprintf("Description: %s", template.Description),

		// Target info
		fmt.Sprintf("Distribution: %s", template.Distribution),
		fmt.Sprintf("Architecture: %s", template.Architecture),
		fmt.Sprintf("Image type: %s", template.ImageType),

		// Packages (most important for matching)
		fmt.Sprintf("Packages: %s", strings.Join(template.Packages, ", ")),

		// Additional context
		fmt.Sprintf("Has disk configuration: %v", template.HasDisk),
		fmt.Sprintf("Has kernel configuration: %v", template.HasKernel),
	}

	if len(template.Repositories) > 0 {
		parts = append(parts, fmt.Sprintf("Repositories: %s", strings.Join(template.Repositories, ", ")))
	}

	return strings.Join(parts, ". ")
}

// FindRelevantTemplates searches for templates similar to the query
func (rag *TemplateRAG) FindRelevantTemplates(ctx context.Context, query string, topK int) ([]*SearchResult, error) {
	// Generate embedding for user query
	queryEmbedding, err := rag.embeddingClient.GenerateEmbedding(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Calculate similarity scores for all templates
	results := make([]*SearchResult, 0, len(rag.templates))
	for _, template := range rag.templates {
		score := cosineSimilarity(queryEmbedding, template.Embedding)
		results = append(results, &SearchResult{
			Template: template,
			Score:    score,
		})
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Return top K results
	if topK > len(results) {
		topK = len(results)
	}

	return results[:topK], nil
}

// GetTemplatesByUseCase returns all templates for a specific use case
func (rag *TemplateRAG) GetTemplatesByUseCase(useCase string) []*TemplateExample {
	return rag.templatesByUse[useCase]
}

// GetAllUseCases returns all available use cases
func (rag *TemplateRAG) GetAllUseCases() []string {
	useCases := make([]string, 0, len(rag.templatesByUse))
	for useCase := range rag.templatesByUse {
		useCases = append(useCases, useCase)
	}
	sort.Strings(useCases)
	return useCases
}

// cosineSimilarity calculates cosine similarity between two vectors
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0.0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// inferUseCaseFromName extracts use case from template name/description
func inferUseCaseFromName(name string) string {
	name = strings.ToLower(name)

	patterns := map[string][]string{
		"minimal":       {"minimal", "bare", "basic"},
		"edge":          {"edge", "cloud-init"},
		"dlstreamer":    {"dlstreamer", "dl-streamer", "video", "streaming"},
		"web-server":    {"web", "nginx", "apache", "http"},
		"database":      {"database", "db", "postgres", "mysql"},
		"container":     {"container", "docker", "kubernetes"},
		"ai-inference":  {"inference", "ai", "ml", "openvino"},
		"embedded":      {"embedded", "iot"},
	}

	for useCase, keywords := range patterns {
		for _, keyword := range keywords {
			if strings.Contains(name, keyword) {
				return useCase
			}
		}
	}

	return "general"
}

// inferUseCaseFromFilename extracts use case from filename
func inferUseCaseFromFilename(filename string) string {
	filename = strings.ToLower(filename)

	if strings.Contains(filename, "minimal") {
		return "minimal"
	}
	if strings.Contains(filename, "edge") {
		return "edge"
	}
	if strings.Contains(filename, "dlstreamer") {
		return "dlstreamer"
	}
	if strings.Contains(filename, "default") {
		return "general"
	}

	return "general"
}

// FormatTemplateForPrompt formats a template example for inclusion in LLM prompt
func (rag *TemplateRAG) FormatTemplateForPrompt(template *TemplateExample, includeFullContent bool) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("### Template: %s\n", template.Name))
	sb.WriteString(fmt.Sprintf("**Use Case**: %s\n", template.UseCase))
	sb.WriteString(fmt.Sprintf("**Description**: %s\n", template.Description))
	sb.WriteString(fmt.Sprintf("**Distribution**: %s | **Architecture**: %s | **Image Type**: %s\n",
		template.Distribution, template.Architecture, template.ImageType))

	if len(template.Packages) > 0 {
		sb.WriteString(fmt.Sprintf("**Packages** (%d): %s\n",
			len(template.Packages), strings.Join(template.Packages[:min(10, len(template.Packages))], ", ")))
		if len(template.Packages) > 10 {
			sb.WriteString(fmt.Sprintf("... and %d more\n", len(template.Packages)-10))
		}
	}

	if len(template.Repositories) > 0 {
		sb.WriteString(fmt.Sprintf("**Custom Repositories**: %s\n", strings.Join(template.Repositories, ", ")))
	}

	if includeFullContent {
		sb.WriteString("\n**Full Template**:\n```yaml\n")
		sb.WriteString(template.Content)
		sb.WriteString("\n```\n")
	}

	return sb.String()
}
