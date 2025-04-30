package validate

import (
    "os"
    "path/filepath"
    "testing"
)

// loadFile reads a test JSON file from the project root testdata directory.
func loadFile(t *testing.T, relPath string) []byte {
    t.Helper()
    // Determine project root relative to this test file
    root := filepath.Join("..", "..")
    fullPath := filepath.Join(root, relPath)
    data, err := os.ReadFile(fullPath)
    if err != nil {
        t.Fatalf("failed to read file %s: %v", fullPath, err)
    }
    return data
}

func TestValid(t *testing.T) {
    v := loadFile(t, "testdata/valid.json")
    if err := ValidateJSON(v); err != nil {
        t.Errorf("expected valid.json to pass, but got: %v", err)
    }
}

func TestInvalid(t *testing.T) {
    v := loadFile(t, "testdata/invalid.json")
    if err := ValidateJSON(v); err == nil {
        t.Errorf("expected invalid.json to fail validation")
    }
}