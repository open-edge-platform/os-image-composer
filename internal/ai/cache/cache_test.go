package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewCache(t *testing.T) {
	tmpDir := t.TempDir()

	cache, err := NewCache(tmpDir)
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}

	// Verify directories were created
	vectorsDir := filepath.Join(tmpDir, "embeddings", "vectors")
	if _, err := os.Stat(vectorsDir); os.IsNotExist(err) {
		t.Error("vectors directory was not created")
	}

	// Verify initial state
	stats := cache.Stats()
	if stats.EntryCount != 0 {
		t.Errorf("expected 0 entries, got %d", stats.EntryCount)
	}
}

func TestComputeContentHash(t *testing.T) {
	content := []byte("test content for hashing")
	hash := ComputeContentHash(content)

	if len(hash) != 16 {
		t.Errorf("expected hash length 16, got %d", len(hash))
	}

	// Same content should produce same hash
	hash2 := ComputeContentHash(content)
	if hash != hash2 {
		t.Error("same content produced different hashes")
	}

	// Different content should produce different hash
	hash3 := ComputeContentHash([]byte("different content"))
	if hash == hash3 {
		t.Error("different content produced same hash")
	}
}

func TestCachePutAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewCache(tmpDir)
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}

	contentHash := "abcd1234efgh5678"
	modelID := "nomic-embed-text"
	templateName := "test-template.yml"
	dimensions := 768
	embedding := []float32{0.1, 0.2, 0.3, 0.4, 0.5}

	// Put embedding
	if err := cache.Put(contentHash, modelID, templateName, dimensions, embedding); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Get embedding
	retrieved, ok := cache.Get(contentHash, modelID)
	if !ok {
		t.Fatal("Get returned false, expected true")
	}

	if len(retrieved) != len(embedding) {
		t.Fatalf("expected %d dimensions, got %d", len(embedding), len(retrieved))
	}

	for i := range embedding {
		if retrieved[i] != embedding[i] {
			t.Errorf("embedding[%d] = %f, expected %f", i, retrieved[i], embedding[i])
		}
	}

	// Verify stats
	stats := cache.Stats()
	if stats.EntryCount != 1 {
		t.Errorf("expected 1 entry, got %d", stats.EntryCount)
	}
	if stats.ModelID != modelID {
		t.Errorf("expected model ID %s, got %s", modelID, stats.ModelID)
	}
	if stats.Dimensions != dimensions {
		t.Errorf("expected dimensions %d, got %d", dimensions, stats.Dimensions)
	}
}

func TestCacheMissForDifferentModel(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewCache(tmpDir)
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}

	contentHash := "abcd1234efgh5678"
	embedding := []float32{0.1, 0.2, 0.3}

	// Put with one model
	if err := cache.Put(contentHash, "nomic-embed-text", "test.yml", 768, embedding); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Get with different model should miss
	_, ok := cache.Get(contentHash, "different-model")
	if ok {
		t.Error("expected cache miss for different model")
	}
}

func TestCacheMissForNonexistent(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewCache(tmpDir)
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}

	_, ok := cache.Get("nonexistent-hash", "nomic-embed-text")
	if ok {
		t.Error("expected cache miss for nonexistent hash")
	}
}

func TestCacheClear(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewCache(tmpDir)
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}

	// Add some entries
	embedding := []float32{0.1, 0.2, 0.3}
	if err := cache.Put("hash1", "model", "t1.yml", 3, embedding); err != nil {
		t.Fatalf("Put failed: %v", err)
	}
	if err := cache.Put("hash2", "model", "t2.yml", 3, embedding); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Verify entries exist
	stats := cache.Stats()
	if stats.EntryCount != 2 {
		t.Errorf("expected 2 entries, got %d", stats.EntryCount)
	}

	// Clear cache
	if err := cache.Clear(); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	// Verify cache is empty
	stats = cache.Stats()
	if stats.EntryCount != 0 {
		t.Errorf("expected 0 entries after clear, got %d", stats.EntryCount)
	}

	// Verify get returns miss
	_, ok := cache.Get("hash1", "model")
	if ok {
		t.Error("expected cache miss after clear")
	}
}

func TestCacheModelChange(t *testing.T) {
	tmpDir := t.TempDir()
	cache, err := NewCache(tmpDir)
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}

	embedding1 := []float32{0.1, 0.2, 0.3}
	embedding2 := []float32{0.4, 0.5, 0.6, 0.7}

	// Put with first model
	if err := cache.Put("hash1", "model1", "t1.yml", 3, embedding1); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Put with different model should clear cache
	if err := cache.Put("hash2", "model2", "t2.yml", 4, embedding2); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// First entry should be gone
	_, ok := cache.Get("hash1", "model2")
	if ok {
		t.Error("expected first entry to be cleared after model change")
	}

	// Second entry should exist
	retrieved, ok := cache.Get("hash2", "model2")
	if !ok {
		t.Error("expected second entry to exist")
	}

	if len(retrieved) != len(embedding2) {
		t.Errorf("expected %d dimensions, got %d", len(embedding2), len(retrieved))
	}
}

func TestCachePersistence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create cache and add entry
	cache1, err := NewCache(tmpDir)
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}

	embedding := []float32{0.1, 0.2, 0.3, 0.4, 0.5}
	if err := cache1.Put("persistent-hash", "model", "test.yml", 5, embedding); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Create new cache instance and verify entry persists
	cache2, err := NewCache(tmpDir)
	if err != nil {
		t.Fatalf("NewCache failed: %v", err)
	}

	retrieved, ok := cache2.Get("persistent-hash", "model")
	if !ok {
		t.Fatal("expected entry to persist across cache instances")
	}

	if len(retrieved) != len(embedding) {
		t.Fatalf("expected %d dimensions, got %d", len(embedding), len(retrieved))
	}

	for i := range embedding {
		if retrieved[i] != embedding[i] {
			t.Errorf("embedding[%d] = %f, expected %f", i, retrieved[i], embedding[i])
		}
	}
}

func TestLoadSaveEmbedding(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.bin")

	original := []float32{0.123, -0.456, 0.789, 1.0, -1.0, 0.0}

	if err := saveEmbedding(path, original); err != nil {
		t.Fatalf("saveEmbedding failed: %v", err)
	}

	loaded, err := loadEmbedding(path)
	if err != nil {
		t.Fatalf("loadEmbedding failed: %v", err)
	}

	if len(loaded) != len(original) {
		t.Fatalf("expected %d dimensions, got %d", len(original), len(loaded))
	}

	for i := range original {
		if loaded[i] != original[i] {
			t.Errorf("embedding[%d] = %f, expected %f", i, loaded[i], original[i])
		}
	}
}
