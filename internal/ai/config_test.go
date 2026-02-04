package ai

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Provider != ProviderOllama {
		t.Errorf("expected provider %s, got %s", ProviderOllama, cfg.Provider)
	}

	if cfg.TemplatesDir != DefaultTemplatesDir {
		t.Errorf("expected templates dir %s, got %s", DefaultTemplatesDir, cfg.TemplatesDir)
	}

	if cfg.Ollama.BaseURL != DefaultOllamaBaseURL {
		t.Errorf("expected Ollama base URL %s, got %s", DefaultOllamaBaseURL, cfg.Ollama.BaseURL)
	}

	if cfg.Ollama.Model != DefaultOllamaModel {
		t.Errorf("expected Ollama model %s, got %s", DefaultOllamaModel, cfg.Ollama.Model)
	}

	if cfg.Ollama.EmbeddingModel != DefaultOllamaEmbeddingModel {
		t.Errorf("expected Ollama embedding model %s, got %s", DefaultOllamaEmbeddingModel, cfg.Ollama.EmbeddingModel)
	}

	if cfg.OpenAI.Model != DefaultOpenAIModel {
		t.Errorf("expected OpenAI model %s, got %s", DefaultOpenAIModel, cfg.OpenAI.Model)
	}

	if cfg.OpenAI.EmbeddingModel != DefaultOpenAIEmbeddingModel {
		t.Errorf("expected OpenAI embedding model %s, got %s", DefaultOpenAIEmbeddingModel, cfg.OpenAI.EmbeddingModel)
	}

	if !cfg.Cache.Enabled {
		t.Error("expected cache to be enabled by default")
	}

	if cfg.Cache.Dir != DefaultCacheDir {
		t.Errorf("expected cache dir %s, got %s", DefaultCacheDir, cfg.Cache.Dir)
	}

	if cfg.Scoring.Semantic != 0.70 {
		t.Errorf("expected semantic weight 0.70, got %f", cfg.Scoring.Semantic)
	}

	if cfg.Scoring.Keyword != 0.20 {
		t.Errorf("expected keyword weight 0.20, got %f", cfg.Scoring.Keyword)
	}

	if cfg.Scoring.Package != 0.10 {
		t.Errorf("expected package weight 0.10, got %f", cfg.Scoring.Package)
	}
}

func TestConfigMerge(t *testing.T) {
	defaults := DefaultConfig()

	// Test merging with empty config
	empty := Config{}
	merged := empty.Merge(defaults)

	if merged.Provider != defaults.Provider {
		t.Errorf("expected provider %s, got %s", defaults.Provider, merged.Provider)
	}

	if merged.Ollama.BaseURL != defaults.Ollama.BaseURL {
		t.Errorf("expected Ollama base URL %s, got %s", defaults.Ollama.BaseURL, merged.Ollama.BaseURL)
	}

	// Test merging with partial config
	partial := Config{
		Provider: ProviderOpenAI,
		Ollama: OllamaConfig{
			BaseURL: "http://custom:11434",
		},
	}
	merged = partial.Merge(defaults)

	if merged.Provider != ProviderOpenAI {
		t.Errorf("expected provider %s, got %s", ProviderOpenAI, merged.Provider)
	}

	if merged.Ollama.BaseURL != "http://custom:11434" {
		t.Errorf("expected custom Ollama base URL, got %s", merged.Ollama.BaseURL)
	}

	// Other fields should be defaults
	if merged.Ollama.Model != defaults.Ollama.Model {
		t.Errorf("expected Ollama model %s, got %s", defaults.Ollama.Model, merged.Ollama.Model)
	}
}

func TestGetEmbeddingModel(t *testing.T) {
	tests := []struct {
		name     string
		provider ProviderType
		expected string
	}{
		{
			name:     "ollama provider",
			provider: ProviderOllama,
			expected: DefaultOllamaEmbeddingModel,
		},
		{
			name:     "openai provider",
			provider: ProviderOpenAI,
			expected: DefaultOpenAIEmbeddingModel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Provider = tt.provider
			if got := cfg.GetEmbeddingModel(); got != tt.expected {
				t.Errorf("GetEmbeddingModel() = %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestGetChatModel(t *testing.T) {
	tests := []struct {
		name     string
		provider ProviderType
		expected string
	}{
		{
			name:     "ollama provider",
			provider: ProviderOllama,
			expected: DefaultOllamaModel,
		},
		{
			name:     "openai provider",
			provider: ProviderOpenAI,
			expected: DefaultOpenAIModel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Provider = tt.provider
			if got := cfg.GetChatModel(); got != tt.expected {
				t.Errorf("GetChatModel() = %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestGetTimeout(t *testing.T) {
	tests := []struct {
		name     string
		provider ProviderType
		expected time.Duration
	}{
		{
			name:     "ollama provider",
			provider: ProviderOllama,
			expected: 120 * time.Second,
		},
		{
			name:     "openai provider",
			provider: ProviderOpenAI,
			expected: 60 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Provider = tt.provider
			if got := cfg.GetTimeout(); got != tt.expected {
				t.Errorf("GetTimeout() = %v, want %v", got, tt.expected)
			}
		})
	}
}
